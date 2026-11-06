// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package sshsession

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// aliasPattern matches ALIAS_PATTERN in ui/src/main/sshHosts.ts. The alias reaches ssh as an
// argument, so one starting with "-" would be read as an option.
var aliasPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidAlias reports whether alias is a host name the helper may pass to ssh.
func ValidAlias(alias string) bool { return aliasPattern.MatchString(alias) }

// stderrCap keeps only the head of ssh's stderr, enough for the reason a login failed.
const stderrCap = 4096

// sftpServerPaths are where distributions install OpenSSH's sftp-server, searched in order by
// sudo mode, which cannot use the sftp subsystem because the subsystem runs as the login user.
var sftpServerPaths = []string{
	"/usr/lib/openssh/sftp-server",
	"/usr/libexec/openssh/sftp-server",
	"/usr/lib/ssh/sftp-server",
	"/usr/libexec/sftp-server",
}

// noSftpServerMarker opens the line the sudo script prints when no sftp-server exists; the
// classifier keys on it, so the two must change together.
const noSftpServerMarker = "no sftp-server at "

// sudoScript execs the first sftp-server found through `sudo -n`, which fails instead of
// prompting, so a sudo that wants a password ends the stream with its own message.
func sudoScript(paths []string) string {
	return `for p in ` + strings.Join(paths, " ") + `; do if [ -x "$p" ]; then exec sudo -n "$p"; fi; done; ` +
		`echo "` + noSftpServerMarker + strings.Join(paths, ", ") + `" >&2; exit 127`
}

// sudoCommand is the remote command line for sudo mode, wrapped in sh -c so a fish or csh login
// shell still runs the POSIX script; the script holds no single quote, so the quoting is safe.
func sudoCommand(paths []string) string {
	return "sh -c '" + sudoScript(paths) + "'"
}

// sessionArgs is the ssh argv for the SFTP stream. Sudo mode runs a command instead of the
// subsystem, since the subsystem cannot be wrapped in sudo.
func sessionArgs(alias, controlPath string, sudo bool) []string {
	args := []string{"-o", "ControlMaster=no", "-o", "ControlPath=" + controlPath, "-o", "BatchMode=yes"}
	if sudo {
		return append(append([]string{"-T"}, args...), "--", alias, sudoCommand(sftpServerPaths))
	}
	return append(args, "-s", "--", alias, "sftp")
}

// sudoLine matches a line sudo or sudo-rs wrote, and a shell's "sudo: not found".
var sudoLine = regexp.MustCompile(`(^|\s)sudo(-rs)?:`)

// classify picks the reply code for a stream that ended before SFTP started, from ssh's stderr.
// It returns "" when nothing is recognised, which the session reports as notLoggedIn.
func classify(stderr string, sudo bool) (code, line string) {
	if !sudo {
		return "", ""
	}
	for _, l := range strings.Split(stderr, "\n") {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, noSftpServerMarker):
			return CodeNoSftpServer, l
		case sudoLine.MatchString(l):
			low := strings.ToLower(l)
			if strings.Contains(low, "password is required") || strings.Contains(low, "authentication is required") {
				return CodeSudoPassword, l
			}
			return CodeSudoRefused, l
		}
	}
	return "", ""
}

// SSHDialer opens SFTP through the tab's ControlMaster, as root through sudo when sudo is set. It
// never logs in by itself (see checkMaster).
func SSHDialer(alias, controlPath string, sudo bool) Dialer {
	return func(ctx context.Context) (io.ReadWriteCloser, error) {
		if err := checkMaster(ctx, alias, controlPath); err != nil {
			return nil, err
		}
		return startPipe(exec.Command("ssh", sessionArgs(alias, controlPath, sudo)...), sudo)
	}
}

// checkMaster fails unless the tab's ControlMaster is up. Every ssh the helper starts checks first,
// because with no master ssh would try a fresh login, which BatchMode can still complete with an
// agent key and so bypass the terminal the human logs in through.
func checkMaster(ctx context.Context, alias, controlPath string) error {
	if !ValidAlias(alias) {
		return fmt.Errorf("invalid host alias %q", alias)
	}
	check := exec.CommandContext(ctx, "ssh", "-o", "ControlPath="+controlPath, "-o", "BatchMode=yes",
		"-O", "check", "--", alias)
	if out, err := check.CombinedOutput(); err != nil {
		return fmt.Errorf("no login for %s yet: %s", alias, firstLine(out, err))
	}
	return nil
}

// SSHRunner runs scripts over the tab's ControlMaster, through `sudo -n` in sudo mode.
func SSHRunner(alias, controlPath string, sudo bool) Runner {
	return func(ctx context.Context, script string) (*exec.Cmd, error) {
		if err := checkMaster(ctx, alias, controlPath); err != nil {
			return nil, err
		}
		return exec.CommandContext(ctx, "ssh", runnerArgs(alias, controlPath, sudo, script)...), nil
	}
}

// runnerArgs wraps script in sh -c for the same reason as sudoCommand.
func runnerArgs(alias, controlPath string, sudo bool, script string) []string {
	remote := "sh -c '" + script + "'"
	if sudo {
		remote = "sudo -n " + remote
	}
	return []string{"-T", "-o", "ControlMaster=no", "-o", "ControlPath=" + controlPath, "-o", "BatchMode=yes",
		"--", alias, remote}
}

// startPipe starts cmd and returns its stdio as the SFTP stream.
func startPipe(cmd *exec.Cmd, sudo bool) (*sshPipe, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &capped{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ssh: %w", err)
	}
	return &sshPipe{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr, sudo: sudo,
		exited: make(chan struct{})}, nil
}

func firstLine(out []byte, err error) string {
	s := strings.TrimSpace(string(out))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return err.Error()
	}
	return s
}

type capped struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *capped) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if room := stderrCap - c.buf.Len(); room > 0 {
		c.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func (c *capped) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.TrimSpace(c.buf.String())
}

// exitWait bounds how long a closed stream waits for ssh to exit, so its stderr is complete.
const exitWait = 2 * time.Second

// sshPipe is ssh's stdio as one stream; Close ends the ssh process.
type sshPipe struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    *capped
	sudo      bool
	exited    chan struct{}
	waitOnce  sync.Once
	closeOnce sync.Once
}

func (p *sshPipe) Write(b []byte) (int, error) { return p.stdin.Write(b) }

// wait reaps ssh. It runs only once stdout has ended or the pipe is closing, since Wait closes
// stdout and would drop bytes not yet read.
func (p *sshPipe) wait() {
	p.waitOnce.Do(func() {
		_ = p.cmd.Wait()
		close(p.exited)
	})
}

// Read turns ssh's exit into its stderr text, so a refused mux session or a sudo refusal reads as
// the reason instead of a bare EOF, carrying a reply code when the reason is recognised.
func (p *sshPipe) Read(b []byte) (int, error) {
	n, err := p.stdout.Read(b)
	if !errors.Is(err, io.EOF) {
		return n, err
	}
	go p.wait()
	select {
	case <-p.exited:
	case <-time.After(exitWait):
	}
	msg := p.stderr.String()
	if code, line := classify(msg, p.sudo); code != "" {
		return n, &codedError{code: code, err: fmt.Errorf("sshsession: %s", line)}
	}
	if msg != "" {
		return n, fmt.Errorf("ssh: %s: %w", firstLine([]byte(msg), err), io.EOF)
	}
	return n, err
}

func (p *sshPipe) Close() error {
	p.closeOnce.Do(func() {
		p.stdin.Close()
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		p.wait()
	})
	return nil
}
