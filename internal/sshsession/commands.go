// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package sshsession

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
)

// Runner returns, not yet started, a command that runs a fixed POSIX sh script on the server, as
// root in sudo mode. The session wires its stdio. A failure means there is no login to run it on.
type Runner func(ctx context.Context, script string) (*exec.Cmd, error)

// The scripts read their paths from stdin, one per line, instead of taking them as arguments: the
// login shell parses whatever ssh puts on the command line, and fish or csh quote differently from
// sh, so the command line holds only these fixed strings. They contain no single quote, since the
// runner wraps them in one, and no "!", which csh expands.
const (
	mvScript  = `IFS= read -r a && IFS= read -r b && exec mv -- "$a" "$b"`
	cpScript  = `IFS= read -r a && IFS= read -r b && exec cp -a -- "$a" "$b"`
	tarScript = `IFS= read -r d && IFS= read -r n && exec tar -C "$d" -czf - -- "$n"`
	// The archive follows the path line on stdin; read consumes only that line, so tar reads the
	// rest. -o leaves the files owned by whoever extracts them, since the source server's user
	// and group ids mean nothing here.
	untarScript = `IFS= read -r d && exec tar -C "$d" -xozf -`
)

// exitError is a remote command that ran and failed.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return "sshsession: " + e.msg }

// command prepares script with paths on its stdin and stderr captured; wait turns its end into an
// error carrying the first stderr line.
type command struct {
	cmd    *exec.Cmd
	stderr *capped
	name   string
}

func (s *Session) command(ctx context.Context, name, script string, paths []string, stdout io.Writer) (*command, error) {
	for _, p := range paths {
		if strings.ContainsAny(p, "\n\r") {
			return nil, fmt.Errorf("sshsession: %s: a path with a line break is not supported", name)
		}
	}
	cmd, err := s.run(ctx, script)
	if err != nil {
		return nil, &codedError{code: CodeNotLoggedIn, err: fmt.Errorf("sshsession: connect: %w", err)}
	}
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\n") + "\n")
	cmd.Stdout = stdout
	stderr := &capped{}
	cmd.Stderr = stderr
	// Killing ssh on cancel can leave a remote child holding stderr; WaitDelay stops Wait from
	// hanging on it.
	cmd.WaitDelay = 2 * time.Second
	return &command{cmd: cmd, stderr: stderr, name: name}, nil
}

func (c *command) wait(err error) error {
	if err == nil {
		return nil
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return fmt.Errorf("sshsession: %s: %w", c.name, err)
	}
	msg := firstLine([]byte(c.stderr.String()), err)
	if c.stderr.String() == "" {
		msg = fmt.Sprintf("%s failed with exit code %d", c.name, ee.ExitCode())
	}
	return &exitError{code: ee.ExitCode(), msg: msg}
}

func (s *Session) runCommand(name, script string, paths ...string) error {
	c, err := s.command(s.ctx, name, script, paths, nil)
	if err != nil {
		return err
	}
	return c.wait(c.cmd.Run())
}

func touch(c *sftp.Client, p string) error {
	if ok, err := taken(c, p); err != nil {
		return fmt.Errorf("sshsession: new file %s: %w", p, err)
	} else if ok {
		return withCode(CodeExists, "sshsession: %s already exists", p)
	}
	f, err := c.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return fmt.Errorf("sshsession: new file %s: %w", p, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("sshsession: new file %s: %w", p, err)
	}
	return nil
}

// move renames over SFTP and falls back to mv, since SFTP rename cannot cross filesystems.
func (s *Session) move(c *sftp.Client, from, to string) error {
	if ok, err := taken(c, to); err != nil {
		return fmt.Errorf("sshsession: move %s: %w", from, err)
	} else if ok {
		return withCode(CodeExists, "sshsession: %s already exists", to)
	}
	if err := c.Rename(from, to); err == nil {
		return nil
	} else if errors.Is(err, sftp.ErrSSHFxConnectionLost) {
		return fmt.Errorf("sshsession: move %s: %w", from, err)
	}
	return s.runCommand("mv", mvScript, from, to)
}

func (s *Session) copy(c *sftp.Client, from, to string) error {
	if ok, err := taken(c, to); err != nil {
		return fmt.Errorf("sshsession: copy %s: %w", from, err)
	} else if ok {
		return withCode(CodeExists, "sshsession: %s already exists", to)
	}
	return s.runCommand("cp", cpScript, from, to)
}

// startDownloadDir streams a gzip tar of a remote folder into a local file. The archive's size is
// unknown until it ends, so total is 0 and progress counts bytes received.
func (s *Session) startDownloadDir(c *sftp.Client, remote, local, id string, replace bool) (any, error) {
	if !filepath.IsAbs(local) {
		return nil, errors.New("sshsession: local path must be absolute")
	}
	if remote == "/" {
		return nil, errors.New("sshsession: cannot archive /")
	}
	fi, err := c.Stat(remote)
	if err != nil {
		return nil, fmt.Errorf("sshsession: download %s: %w", remote, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("sshsession: %s is not a folder", remote)
	}
	if err := checkLocalTarget(local, replace); err != nil {
		return nil, err
	}
	ctx, err := s.register(id)
	if err != nil {
		return nil, err
	}
	dst, err := openLocalTarget(local, replace)
	if err != nil {
		s.unregister(id)
		return nil, err
	}
	m := &meter{s: s, ctx: ctx, id: id, last: time.Now()}
	cmd, err := s.command(ctx, "tar", tarScript, []string{path.Dir(remote), path.Base(remote)}, meterWriter{w: dst.f, m: m})
	if err == nil {
		err = cmd.cmd.Start()
		if err != nil {
			err = fmt.Errorf("sshsession: start ssh: %w", err)
		}
	}
	if err != nil {
		dst.abort()
		s.unregister(id)
		return nil, err
	}
	return &pending{Transfer: id, Total: 0, run: func() {
		defer s.unregister(id)
		err := cmd.wait(cmd.cmd.Wait())
		// GNU tar exits 1 when a file changed while it was read, common under /var/log; the
		// archive is still complete, so only that message counts as success.
		var ee *exitError
		if errors.As(err, &ee) && ee.code == 1 && strings.Contains(cmd.stderr.String(), "file changed as we read it") {
			err = nil
		}
		if err == nil {
			err = dst.commit()
		}
		s.finish(ctx, m, err, dst.abort)
	}}, nil
}

// startUploadDir unpacks a local gzip tar, as startDownloadDir writes it, into the remote folder
// that holds remote, whose base name must be the archive's top entry. Progress counts archive
// bytes sent; a failure or cancel removes what was unpacked.
func (s *Session) startUploadDir(c *sftp.Client, local, remote, id string) (any, error) {
	if !filepath.IsAbs(local) {
		return nil, errors.New("sshsession: local path must be absolute")
	}
	if remote == "/" {
		return nil, errors.New("sshsession: cannot unpack over /")
	}
	fi, err := os.Stat(local)
	if err != nil {
		return nil, fmt.Errorf("sshsession: upload %s: %w", local, err)
	}
	if fi.IsDir() {
		return nil, withCode(CodeIsDir, "sshsession: %s is a folder, not an archive", local)
	}
	if ok, err := taken(c, remote); err != nil {
		return nil, fmt.Errorf("sshsession: upload to %s: %w", remote, err)
	} else if ok {
		return nil, withCode(CodeExists, "sshsession: %s already exists", remote)
	}
	ctx, err := s.register(id)
	if err != nil {
		return nil, err
	}
	src, err := os.Open(local)
	if err != nil {
		s.unregister(id)
		return nil, fmt.Errorf("sshsession: upload %s: %w", local, err)
	}
	m := &meter{s: s, ctx: ctx, id: id, total: fi.Size(), last: time.Now()}
	dir := path.Dir(remote)
	cmd, err := s.command(ctx, "tar", untarScript, []string{dir}, nil)
	if err == nil {
		cmd.cmd.Stdin = io.MultiReader(strings.NewReader(dir+"\n"), meterReader{r: src, m: m})
		err = cmd.cmd.Start()
		if err != nil {
			err = fmt.Errorf("sshsession: start ssh: %w", err)
		}
	}
	if err != nil {
		src.Close()
		s.unregister(id)
		return nil, err
	}
	return &pending{Transfer: id, Total: fi.Size(), run: func() {
		defer s.unregister(id)
		err := cmd.wait(cmd.cmd.Wait())
		src.Close()
		s.finish(ctx, m, err, func() { _ = removeAll(c, remote) })
	}}, nil
}
