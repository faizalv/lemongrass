// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package sshsession

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/sftp"
)

// pending is a transfer whose preflight passed. Its reply goes out before run starts, so the
// drawer always sees the accepted reply before any progress event.
type pending struct {
	Transfer string `json:"transfer"`
	Total    int64  `json:"total"`
	run      func()
}

type progressEvent struct {
	Event    string `json:"event"`
	Transfer string `json:"transfer"`
	Done     int64  `json:"done"`
	Total    int64  `json:"total"`
}

type endEvent struct {
	Event    string `json:"event"`
	Transfer string `json:"transfer"`
	Error    string `json:"error,omitempty"`
	Code     string `json:"code,omitempty"`
}

// meter counts bytes through a transfer, reports progress at most every progressEvery, and stops
// the copy once the transfer is cancelled.
type meter struct {
	s     *Session
	ctx   context.Context
	id    string
	total int64

	mu   sync.Mutex
	done int64
	last time.Time
}

func (m *meter) add(n int) error {
	m.mu.Lock()
	m.done += int64(n)
	emit := time.Since(m.last) >= progressEvery
	if emit {
		m.last = time.Now()
	}
	done := m.done
	m.mu.Unlock()
	if emit {
		m.s.send(progressEvent{Event: "progress", Transfer: m.id, Done: done, Total: m.total})
	}
	return m.ctx.Err()
}

type meterReader struct {
	r io.Reader
	m *meter
}

func (r meterReader) Read(p []byte) (int, error) {
	if err := r.m.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	if merr := r.m.add(n); merr != nil && err == nil {
		err = merr
	}
	return n, err
}

type meterWriter struct {
	w io.Writer
	m *meter
}

func (w meterWriter) Write(p []byte) (int, error) {
	if err := w.m.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := w.w.Write(p)
	if merr := w.m.add(n); merr != nil && err == nil {
		err = merr
	}
	return n, err
}

func (s *Session) register(id string) (context.Context, error) {
	if id == "" {
		return nil, errors.New("sshsession: transfer id is required")
	}
	s.xferMu.Lock()
	defer s.xferMu.Unlock()
	if _, ok := s.transfers[id]; ok {
		return nil, fmt.Errorf("sshsession: transfer %s is already running", id)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.transfers[id] = cancel
	return ctx, nil
}

func (s *Session) unregister(id string) {
	s.xferMu.Lock()
	defer s.xferMu.Unlock()
	if cancel, ok := s.transfers[id]; ok {
		cancel()
		delete(s.transfers, id)
	}
}

// cancel stops a running transfer; an unknown id has already ended, so it is not an error.
func (s *Session) cancel(id string) {
	s.xferMu.Lock()
	defer s.xferMu.Unlock()
	if cancel, ok := s.transfers[id]; ok {
		cancel()
	}
}

// finish reports a transfer's end. Any failure, a cancel included, removes the partial target so
// a half file never looks complete.
func (s *Session) finish(ctx context.Context, m *meter, err error, removePartial func()) {
	if err == nil {
		m.mu.Lock()
		done := m.done
		m.mu.Unlock()
		s.send(progressEvent{Event: "progress", Transfer: m.id, Done: done, Total: m.total})
		s.send(endEvent{Event: "done", Transfer: m.id})
		return
	}
	removePartial()
	if ctx.Err() != nil {
		s.send(endEvent{Event: "failed", Transfer: m.id, Error: "sshsession: cancelled", Code: CodeCancelled})
		return
	}
	s.send(endEvent{Event: "failed", Transfer: m.id, Error: err.Error()})
}

func (s *Session) startUpload(c *sftp.Client, local, remote, id string) (any, error) {
	if !filepath.IsAbs(local) {
		return nil, errors.New("sshsession: local path must be absolute")
	}
	fi, err := os.Stat(local)
	if err != nil {
		return nil, fmt.Errorf("sshsession: upload %s: %w", local, err)
	}
	if fi.IsDir() {
		return nil, withCode(CodeIsDir, "sshsession: %s is a folder", local)
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
	dst, err := c.OpenFile(remote, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		src.Close()
		s.unregister(id)
		return nil, fmt.Errorf("sshsession: upload to %s: %w", remote, err)
	}
	m := &meter{s: s, ctx: ctx, id: id, total: fi.Size(), last: time.Now()}
	return &pending{Transfer: id, Total: fi.Size(), run: func() {
		defer s.unregister(id)
		_, err := dst.ReadFromWithConcurrency(meterReader{r: src, m: m}, 0)
		src.Close()
		if cerr := dst.Close(); err == nil && cerr != nil {
			err = cerr
		}
		if err != nil {
			err = fmt.Errorf("sshsession: upload to %s: %w", remote, err)
		}
		s.finish(ctx, m, err, func() { _ = c.Remove(remote) })
	}}, nil
}

func (s *Session) startDownload(c *sftp.Client, remote, local, id string, replace bool) (any, error) {
	if !filepath.IsAbs(local) {
		return nil, errors.New("sshsession: local path must be absolute")
	}
	fi, err := c.Stat(remote)
	if err != nil {
		return nil, fmt.Errorf("sshsession: download %s: %w", remote, err)
	}
	if fi.IsDir() {
		return nil, withCode(CodeIsDir, "sshsession: %s is a folder", remote)
	}
	if err := checkLocalTarget(local, replace); err != nil {
		return nil, err
	}
	ctx, err := s.register(id)
	if err != nil {
		return nil, err
	}
	src, err := c.Open(remote)
	if err != nil {
		s.unregister(id)
		return nil, fmt.Errorf("sshsession: download %s: %w", remote, err)
	}
	dst, err := openLocalTarget(local, replace)
	if err != nil {
		src.Close()
		s.unregister(id)
		return nil, err
	}
	m := &meter{s: s, ctx: ctx, id: id, total: fi.Size(), last: time.Now()}
	return &pending{Transfer: id, Total: fi.Size(), run: func() {
		defer s.unregister(id)
		_, err := src.WriteTo(meterWriter{w: dst.f, m: m})
		src.Close()
		if err == nil {
			err = dst.commit()
		}
		if err != nil {
			err = fmt.Errorf("sshsession: download to %s: %w", local, err)
		}
		s.finish(ctx, m, err, dst.abort)
	}}, nil
}
