// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Package sshsession is the helper behind an SSH tab's file browser. It speaks SFTP over a
// transport that rides the tab's ControlMaster, and answers JSON-line requests from Electron main:
// one object per line each way, replies matched by id, transfer progress as unprompted events.
package sshsession

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/pkg/sftp"
)

// Dialer opens the SFTP byte stream. A failure means the terminal has not logged in yet, so it is
// reported as notLoggedIn and retried on the next request.
type Dialer func(ctx context.Context) (io.ReadWriteCloser, error)

// Error codes the drawer branches on; any other failure carries no code.
const (
	CodeNotLoggedIn = "notLoggedIn"
	CodeExists      = "exists"
	CodeIsDir       = "isDir"
	CodeCancelled   = "cancelled"
	// Sudo mode only: sudo wants a password, no sftp-server binary was found, or sudo refused for
	// another reason (not in sudoers, not installed, requiretty).
	CodeSudoPassword = "sudoPassword"
	CodeNoSftpServer = "noSftpServer"
	CodeSudoRefused  = "sudoRefused"
	// Editor: the file is not UTF-8 text, is over maxEditBytes, or moved on the server since read.
	CodeBinary   = "binary"
	CodeTooLarge = "tooLarge"
	CodeChanged  = "changed"
)

// A request line carries at most an edited file (maxEditBytes) after JSON escaping; the cap bounds
// memory on a bad line.
const maxLineBytes = 16 << 20

// dialTimeout bounds the SFTP handshake, so a hung ssh does not hold every request.
const dialTimeout = 20 * time.Second

// progressEvery is how often a transfer reports progress; the drawer only redraws a bar.
var progressEvery = 200 * time.Millisecond

// Session serves one tab's requests over one lazily opened SFTP client, and runs the operations
// SFTP has no verb for (move across filesystems, copy, folder archive) as commands over the master.
type Session struct {
	dial Dialer
	run  Runner

	// ctx ends with the session, so a long copy or archive does not outlive the tab.
	ctx  context.Context
	stop context.CancelFunc

	outMu sync.Mutex
	out   *json.Encoder

	connMu sync.Mutex
	client *sftp.Client
	home   string
	dead   chan struct{} // closed once client's connection has shut down

	xferMu    sync.Mutex
	transfers map[string]context.CancelFunc

	work sync.WaitGroup
}

// New makes a session that dials with d on its first request and runs commands with r.
func New(d Dialer, r Runner) *Session {
	ctx, stop := context.WithCancel(context.Background())
	return &Session{dial: d, run: r, ctx: ctx, stop: stop, transfers: map[string]context.CancelFunc{}}
}

type request struct {
	ID       json.RawMessage `json:"id"`
	Op       string          `json:"op"`
	Path     string          `json:"path"`
	From     string          `json:"from"`
	To       string          `json:"to"`
	Remote   string          `json:"remote"`
	Local    string          `json:"local"`
	Transfer string          `json:"transfer"`
	Replace  bool            `json:"replace"`
	Content  string          `json:"content"`
	Mtime    int64           `json:"mtime"`
	Force    bool            `json:"force"`
}

type reply struct {
	ID     json.RawMessage `json:"id"`
	OK     bool            `json:"ok"`
	Result any             `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
	Code   string          `json:"code,omitempty"`
}

// codedError carries a protocol code with its message.
type codedError struct {
	code string
	err  error
}

func (e *codedError) Error() string { return e.err.Error() }
func (e *codedError) Unwrap() error { return e.err }

func withCode(code string, format string, args ...any) error {
	return &codedError{code: code, err: fmt.Errorf(format, args...)}
}

// Serve reads requests from in until it closes, then cancels running transfers, waits for them
// to clean up and closes the SFTP client, which ends ssh.
func (s *Session) Serve(in io.Reader, out io.Writer) error {
	s.out = json.NewEncoder(out)
	s.out.SetEscapeHTML(false)
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 64*1024), maxLineBytes)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			s.send(reply{ID: req.ID, Error: "sshsession: bad request: " + err.Error()})
			continue
		}
		s.work.Add(1)
		go func() {
			defer s.work.Done()
			s.handle(req)
		}()
	}
	s.shutdown()
	if err := sc.Err(); err != nil {
		return fmt.Errorf("sshsession: read requests: %w", err)
	}
	return nil
}

func (s *Session) shutdown() {
	s.xferMu.Lock()
	for _, cancel := range s.transfers {
		cancel()
	}
	s.xferMu.Unlock()
	done := make(chan struct{})
	go func() { s.work.Wait(); close(done) }()
	// Cancelled transfers remove their partial files over the client, so it stays open briefly;
	// closing it after that unblocks anything still waiting on a stalled connection.
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
	s.connMu.Lock()
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}
	s.connMu.Unlock()
	s.stop()
	<-done
}

func (s *Session) send(v any) {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	_ = s.out.Encode(v)
}

// lostGrace is how long a failed request waits to learn whether the connection itself went down;
// a write to a dead ssh fails before the client's reader notices the closed stream.
const lostGrace = 200 * time.Millisecond

func (s *Session) handle(req request) {
	result, dead, err := s.do(req)
	if err != nil {
		r := reply{ID: req.ID, Error: err.Error()}
		var ce *codedError
		if errors.As(err, &ce) {
			r.Code = ce.code
		} else if errors.Is(err, sftp.ErrSSHFxConnectionLost) {
			r.Code = CodeNotLoggedIn
		} else if dead != nil {
			// The master going away (the terminal logged out) reads as notLoggedIn, so the
			// drawer offers Retry, which redials.
			select {
			case <-dead:
				r.Code = CodeNotLoggedIn
			case <-time.After(lostGrace):
			}
		}
		s.send(r)
		return
	}
	if result == nil {
		result = struct{}{}
	}
	s.send(reply{ID: req.ID, OK: true, Result: result})
	if p, ok := result.(*pending); ok {
		p.run()
	}
}

func (s *Session) do(req request) (any, <-chan struct{}, error) {
	if req.Op == "cancel" {
		s.cancel(req.Transfer)
		return nil, nil, nil
	}
	c, home, dead, err := s.conn()
	if err != nil {
		return nil, nil, err
	}
	result, err := s.op(c, home, req)
	return result, dead, err
}

func (s *Session) op(c *sftp.Client, home string, req request) (any, error) {
	switch req.Op {
	case "home":
		return map[string]string{"path": home}, nil
	case "list":
		return list(c, resolve(home, req.Path))
	case "mkdir":
		return nil, mkdir(c, resolve(home, req.Path))
	case "rename":
		return nil, rename(c, resolve(home, req.From), resolve(home, req.To))
	case "remove":
		return nil, remove(c, resolve(home, req.Path))
	case "read":
		return readText(c, resolve(home, req.Path))
	case "write":
		return writeText(c, resolve(home, req.Path), req.Content, req.Mtime, req.Force)
	case "touch":
		return nil, touch(c, resolve(home, req.Path))
	case "move":
		return nil, s.move(c, resolve(home, req.From), resolve(home, req.To))
	case "copy":
		return nil, s.copy(c, resolve(home, req.From), resolve(home, req.To))
	case "download":
		return s.startDownload(c, resolve(home, req.Remote), req.Local, req.Transfer, req.Replace)
	case "downloadDir":
		return s.startDownloadDir(c, resolve(home, req.Remote), req.Local, req.Transfer, req.Replace)
	case "upload":
		return s.startUpload(c, req.Local, resolve(home, req.Remote), req.Transfer)
	default:
		return nil, fmt.Errorf("sshsession: unknown op %q", req.Op)
	}
}

// conn returns the live client, dialing when there is none or the last one has dropped.
func (s *Session) conn() (*sftp.Client, string, <-chan struct{}, error) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	if s.client != nil {
		return s.client, s.home, s.dead, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	rwc, err := s.dial(ctx)
	if err != nil {
		return nil, "", nil, &codedError{code: CodeNotLoggedIn, err: fmt.Errorf("sshsession: connect: %w", err)}
	}
	// The handshake and the home lookup are the steps that can hang; closing the stream on timeout
	// unblocks both.
	stop := context.AfterFunc(ctx, func() { rwc.Close() })
	c, err := sftp.NewClientPipe(rwc, rwc)
	if err != nil {
		rwc.Close()
		var ce *codedError
		if errors.As(err, &ce) {
			return nil, "", nil, ce
		}
		return nil, "", nil, &codedError{code: CodeNotLoggedIn, err: fmt.Errorf("sshsession: start sftp: %w", err)}
	}
	home, err := c.RealPath(".")
	if err == nil && !stop() {
		err = errors.New("timed out")
	}
	if err != nil {
		c.Close()
		return nil, "", nil, &codedError{code: CodeNotLoggedIn, err: fmt.Errorf("sshsession: home folder: %w", err)}
	}
	dead := make(chan struct{})
	s.client, s.home, s.dead = c, home, dead
	go func() {
		_ = c.Wait()
		s.connMu.Lock()
		if s.client == c {
			s.client = nil
		}
		s.connMu.Unlock()
		close(dead)
	}()
	return c, home, dead, nil
}
