package usecase

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/faizalv/lemongrass/modules/pty/entity"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b[()][A-Z0-9]|\x1b[=>]|\x1b[^\[]`)

type PtyUsecase struct {
	log     *log.Logger
	logFile *os.File
}

func New(runnerLogPath string) *PtyUsecase {
	var w io.Writer = os.Stderr
	var logFile *os.File
	if f, err := os.OpenFile(runnerLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		w = io.MultiWriter(os.Stderr, f)
		logFile = f
	}
	return &PtyUsecase{log: log.New(w, "[pty] ", log.LstdFlags), logFile: logFile}
}

func (u *PtyUsecase) Close() {
	if u.logFile != nil {
		u.logFile.Close()
	}
}

func (u *PtyUsecase) RunTest() (entity.Session, error) {
	return u.Run("echo hello from lemongrass")
}

func (u *PtyUsecase) Run(prompt string) (entity.Session, error) {
	u.log.Printf("session start prompt=%q", prompt)

	// script(1) allocates a pty for claude and bridges it to our stdin/stdout
	// pipe using poll(2), so stdin writes reach claude regardless of whether
	// our caller has a real terminal. unbuffer uses Tcl's `interact` which
	// silently drops input when stdin is a pipe, which is why it didn't work.
	// -q: suppress start/end messages  -f: flush after every write (real-time)
	cmd := exec.Command("docker", "exec", "-i", "lg-runner",
		"script", "-qf", "-c", "claude", "/dev/null")

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return entity.Session{}, fmt.Errorf("stdin pipe: %w", err)
	}

	// Single pipe for stdout+stderr so we read one stream.
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		u.log.Printf("start error: %v", err)
		return entity.Session{}, fmt.Errorf("start: %w", err)
	}

	// Close the write end once the process exits so the reader gets EOF.
	go func() {
		cmd.Wait()
		pw.Close()
	}()

	out := &outputBuffer{log: u.log}
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
				u.log.Printf("reader exit: %v", readErr)
				return
			}
		}
	}()

	// Claude shows a "trust this folder" prompt on first run in a directory.
	// Detect it and press Enter to accept before sending the real prompt.
	u.log.Printf("waiting for trust prompt (up to 10s)...")
	if out.waitFor("trust", 10*time.Second) {
		u.log.Printf("trust prompt detected — pressing Enter")
		stdinPipe.Write([]byte("\r"))
		time.Sleep(500 * time.Millisecond)
	} else {
		u.log.Printf("no trust prompt seen, continuing")
	}

	// Wait until claude's shell is ready — any of these strings means it's at
	// the prompt and accepting input.
	// Wait for all 3 signals so we inject only after the welcome screen has
	// fully rendered. "forshortcuts" appears immediately on startup (too early
	// alone); "Welcomeback" and "ClaudeCode" only appear once the welcome box
	// draws — confirmed from log output.
	readySignals := []string{"forshortcuts", "Welcomeback", "ClaudeCode"}
	u.log.Printf("waiting for claude ready (need all 3 signals)...")
	if ok := out.waitForAll(readySignals, 30*time.Second); ok {
		u.log.Printf("claude ready (all signals detected)")
	} else {
		u.log.Printf("ready signal timeout — injecting anyway")
	}

	u.log.Printf("injecting: %q", prompt)
	if _, err := stdinPipe.Write([]byte(prompt + "\r")); err != nil {
		cmd.Process.Kill()
		<-readDone
		return entity.Session{}, fmt.Errorf("write prompt: %w", err)
	}

	// The PTY raw bytes arrive in a burst so waitForAll can fire before the
	// welcome screen finishes rendering (~10s). The first \r above may be
	// consumed during that init. Send a second \r after 15s to guarantee
	// submission once the input box is live.
	u.log.Printf("waiting for welcome screen to settle (15s)...")
	time.Sleep(15 * time.Second)
	u.log.Printf("sending confirmation \\r")
	stdinPipe.Write([]byte("\r"))

	u.log.Printf("waiting for response (60s)...")
	time.Sleep(60 * time.Second)

	u.log.Printf("terminating session")
	stdinPipe.Write([]byte{3}) // Ctrl+C
	time.Sleep(300 * time.Millisecond)
	stdinPipe.Write([]byte{4}) // Ctrl+D
	stdinPipe.Close()
	cmd.Process.Kill()

	<-readDone

	result := out.clean()
	u.log.Printf("session done, output=%d chars", len(result))
	return entity.Session{ID: "test", Output: result}, nil
}

// outputBuffer accumulates raw output, logs clean lines, and supports polling.
type outputBuffer struct {
	mu  sync.Mutex
	raw []byte
	log *log.Logger
}

func (o *outputBuffer) write(p []byte) {
	clean := strings.TrimSpace(ansiEscape.ReplaceAllString(string(p), ""))
	if clean != "" {
		o.log.Printf("out: %s", clean)
	}
	o.mu.Lock()
	o.raw = append(o.raw, p...)
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

func (o *outputBuffer) clean() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return ansiEscape.ReplaceAllString(string(o.raw), "")
}
