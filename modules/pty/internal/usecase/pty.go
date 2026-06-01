package usecase

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/faizalv/lemongrass/infra"
	"github.com/faizalv/lemongrass/modules/pty/entity"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b[()][A-Z0-9]|\x1b[=>]|\x1b[^\[]`)

type PtyUsecase struct {
	log         *log.Logger
	logW        io.Closer
	sessionsDir string
}

func New(logDir string) *PtyUsecase {
	sessionsDir := filepath.Join(logDir, "sessions")
	os.MkdirAll(sessionsDir, 0755)
	w := infra.NewDailyRotateWriter(logDir, "runner", 7)
	return &PtyUsecase{
		log:         log.New(io.MultiWriter(os.Stderr, w), "[pty] ", log.LstdFlags),
		logW:        w,
		sessionsDir: sessionsDir,
	}
}

func (u *PtyUsecase) Close() {
	if u.logW != nil {
		u.logW.Close()
	}
}

type Session struct {
	stdinPipe   io.WriteCloser
	cmd         *exec.Cmd
	done        chan struct{}
	readDone    chan struct{}
	out         *outputBuffer
	promptFile  string   // temp file in runner, cleaned up on Close
	sessionFile *os.File // per-session output log, closed on Close
}

func (s *Session) Write(b []byte) (int, error) {
	return s.stdinPipe.Write(b)
}

func (s *Session) WaitIdle(quiesce, max time.Duration) {
	s.out.waitForIdle(quiesce, max)
}

func (s *Session) Output() string {
	return s.out.clean()
}

func (s *Session) Close() {
	s.stdinPipe.Write([]byte{3}) // Ctrl+C -- claude prompts "press again to exit"
	time.Sleep(300 * time.Millisecond)
	s.stdinPipe.Write([]byte{3}) // Ctrl+C -- confirms exit
	time.Sleep(300 * time.Millisecond)
	s.stdinPipe.Write([]byte{4}) // Ctrl+D -- EOF fallback
	s.stdinPipe.Close()
	s.cmd.Process.Kill()
	<-s.done
	<-s.readDone
	if s.promptFile != "" {
		exec.Command("docker", "exec", "lg-runner", "rm", "-f", s.promptFile).Run()
	}
	if s.sessionFile != nil {
		s.sessionFile.Close()
	}
}

func (u *PtyUsecase) Open(systemPrompt, sessionID, sessionType string) (*Session, error) {
	u.log.Printf("open: sessionID=%q sessionType=%q promptLen=%d", sessionID, sessionType, len(systemPrompt))

	// write system prompt to a temp file in the runner so we avoid shell
	// escaping entirely, then reference it via $(cat ...) in the claude command.
	// each session gets a unique file so concurrent sessions don't collide.
	var promptFile string
	claudeCmd := "claude"
	if systemPrompt != "" {
		promptFile = fmt.Sprintf("/tmp/lg-prompt-%d.txt", time.Now().UnixNano())
		preview := systemPrompt
		if len(preview) > 300 {
			preview = preview[:300] + "…"
		}
		u.log.Printf("writing system prompt to %s (%d bytes)", promptFile, len(systemPrompt))
		u.log.Printf("prompt preview: %s", strings.ReplaceAll(preview, "\n", "\\n"))
		t0 := time.Now()
		writeCmd := exec.Command("docker", "exec", "-i", "lg-runner",
			"sh", "-c", fmt.Sprintf("cat > %s", promptFile))
		writeCmd.Stdin = strings.NewReader(systemPrompt)
		if err := writeCmd.Run(); err != nil {
			return nil, fmt.Errorf("write system prompt: %w", err)
		}
		u.log.Printf("system prompt written OK (%.0fms)", float64(time.Since(t0).Milliseconds()))
		claudeCmd = fmt.Sprintf(`claude --append-system-prompt "$(cat %s)"`, promptFile)
	}

	// script(1) allocates a pty for claude and bridges it to our stdin/stdout
	// pipe using poll(2), so stdin writes reach claude regardless of whether
	// our caller has a real terminal. unbuffer uses Tcl's `interact` which
	// silently drops input when stdin is a pipe, which is why it didn't work.
	// -q: suppress start/end messages  -f: flush after every write (real-time)
	execArgs := []string{"exec", "-i"}
	if sessionID != "" {
		execArgs = append(execArgs, "-e", "LG_SESSION_ID="+sessionID)
		u.log.Printf("env: LG_SESSION_ID=%s", sessionID)
	}
	if sessionType != "" {
		execArgs = append(execArgs, "-e", "LG_SESSION_TYPE="+sessionType)
		u.log.Printf("env: LG_SESSION_TYPE=%s", sessionType)
	}
	execArgs = append(execArgs, "lg-runner", "script", "-qf", "-c", claudeCmd, "/dev/null")
	u.log.Printf("exec: docker %s", strings.Join(execArgs, " "))
	cmd := exec.Command("docker", execArgs...)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	// Single pipe for stdout+stderr so we read one stream.
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		return nil, fmt.Errorf("start: %w", err)
	}

	// Close the write end once the process exits so the reader gets EOF.
	done := make(chan struct{})
	go func() {
		defer close(done)
		cmd.Wait()
		pw.Close()
	}()

	var sessionFile *os.File
	var sessionLog *log.Logger
	if sessionID != "" && u.sessionsDir != "" {
		stamp := time.Now().Format("20060102-150405")
		sessPath := filepath.Join(u.sessionsDir, sessionID+"-"+stamp+".log")
		if f, err := os.OpenFile(sessPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
			sessionFile = f
			sessionLog = log.New(f, "", log.LstdFlags)
		}
		go cleanupSessionLogs(u.sessionsDir, 7)
	}

	out := &outputBuffer{log: u.log, sessionLog: sessionLog}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 4096)
		for {
			n, readErr := pr.Read(buf)
			if n > 0 {
				out.write(buf[:n])
			}
			if readErr != nil {
				return
			}
		}
	}()

	// Claude shows a "trust this folder" prompt on first run in a directory.
	// Detect it and press Enter to accept before sending the real prompt.
	u.log.Printf("waiting for trust prompt (up to 10s)...")
	t1 := time.Now()
	if out.waitFor("trust", 10*time.Second) {
		u.log.Printf("trust prompt detected (%.0fms), pressing Enter", float64(time.Since(t1).Milliseconds()))
		stdinPipe.Write([]byte("\r"))
		time.Sleep(500 * time.Millisecond)
	} else {
		u.log.Printf("no trust prompt seen (%.0fms), continuing", float64(time.Since(t1).Milliseconds()))
	}

	// Wait until claude's shell is ready; any of these strings means it's at
	// the prompt and accepting input.
	// Wait for all 3 signals so we inject only after the welcome screen has
	// fully rendered. "forshortcuts" appears immediately on startup (too early
	// alone); "Welcomeback" and "ClaudeCode" only appear once the welcome box
	// draws (confirmed from log output).
	readySignals := []string{"forshortcuts", "Welcomeback", "ClaudeCode"}
	u.log.Printf("waiting for claude ready (need all 3 signals)...")
	t2 := time.Now()
	if ok := out.waitForAll(readySignals, 30*time.Second); ok {
		u.log.Printf("claude ready (all signals detected, %.0fms)", float64(time.Since(t2).Milliseconds()))
	} else {
		u.log.Printf("ready signal timeout after %.0fms, injecting anyway", float64(time.Since(t2).Milliseconds()))
	}

	// The PTY raw bytes arrive in a burst so waitForAll can fire before the
	// welcome screen finishes rendering. Wait until output goes quiet (input
	// box is live) before sending the confirmation \r.
	u.log.Printf("waiting for welcome screen to settle (idle 2s, max 20s)...")
	t3 := time.Now()
	out.waitForIdle(2*time.Second, 20*time.Second)
	u.log.Printf("screen settled (%.0fms), sending confirmation \\r", float64(time.Since(t3).Milliseconds()))
	stdinPipe.Write([]byte("\r"))

	u.log.Printf("session ready")
	return &Session{
		stdinPipe:   stdinPipe,
		cmd:         cmd,
		done:        done,
		readDone:    readDone,
		out:         out,
		promptFile:  promptFile,
		sessionFile: sessionFile,
	}, nil
}

func (u *PtyUsecase) RunTest() (entity.Session, error) {
	sess, err := u.Open("", "", "")
	if err != nil {
		return entity.Session{}, err
	}
	sess.WaitIdle(5*time.Second, 5*time.Minute)
	sess.Close()
	return entity.Session{ID: "test", Output: sess.Output()}, nil
}

// outputBuffer accumulates raw output, logs clean lines, and supports polling.
type outputBuffer struct {
	mu         sync.Mutex
	raw        []byte
	lastWrite  time.Time
	log        *log.Logger
	sessionLog *log.Logger // when set, output lines go here instead of the shared log
}

func (o *outputBuffer) write(p []byte) {
	clean := strings.TrimSpace(ansiEscape.ReplaceAllString(string(p), ""))
	if clean != "" {
		if o.sessionLog != nil {
			o.sessionLog.Printf("%s", clean)
		} else {
			o.log.Printf("out: %s", clean)
		}
	}
	o.mu.Lock()
	o.raw = append(o.raw, p...)
	o.lastWrite = time.Now()
	o.mu.Unlock()
}

func (o *outputBuffer) contains(s string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return strings.Contains(
		strings.ToLower(ansiEscape.ReplaceAllString(string(o.raw), "")),
		strings.ToLower(s),
	)
}

func (o *outputBuffer) waitFor(s string, timeout time.Duration) bool {
	return o.waitForAny([]string{s}, timeout) != ""
}

func (o *outputBuffer) waitForAny(candidates []string, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, s := range candidates {
			if o.contains(s) {
				return s
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return ""
}

func (o *outputBuffer) waitForAll(signals []string, timeout time.Duration) bool {
	seen := make(map[string]bool)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, s := range signals {
			if !seen[s] && o.contains(s) {
				seen[s] = true
			}
		}
		if len(seen) == len(signals) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// waitForIdle returns true once no new bytes have arrived for quiesce duration,
// or false if maxTimeout is reached first.
func (o *outputBuffer) waitForIdle(quiesce, maxTimeout time.Duration) bool {
	deadline := time.Now().Add(maxTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		o.mu.Lock()
		last := o.lastWrite
		o.mu.Unlock()
		if !last.IsZero() && time.Since(last) >= quiesce {
			return true
		}
	}
	return false
}

func (o *outputBuffer) clean() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return ansiEscape.ReplaceAllString(string(o.raw), "")
}

func cleanupSessionLogs(dir string, maxDays int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -maxDays)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
