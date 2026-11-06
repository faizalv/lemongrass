// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build unix

package sshsession

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The upload reads from a FIFO the test feeds, so the transfer is mid-way, by construction, when
// the cancel arrives.
func TestCancelRemovesPartial(t *testing.T) {
	old := progressEvery
	progressEvery = 0
	t.Cleanup(func() { progressEvery = old })
	h := newHarness(t, nil)
	fifo := filepath.Join(t.TempDir(), "feed")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	// O_RDWR keeps the FIFO open without waiting for a reader, so the helper's open does not block.
	feed, err := os.OpenFile(fifo, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer feed.Close()
	remote := h.path("feed")

	h.ok(msg{"op": "upload", "local": fifo, "remote": remote, "transfer": "c1"})
	if _, err := feed.Write(make([]byte, 64*1024)); err != nil {
		t.Fatal(err)
	}
	h.next(func(m msg) bool { return m["transfer"] == "c1" && m["event"] == "progress" })

	// List runs while the transfer is still open.
	h.ok(msg{"op": "list", "path": h.root})

	h.ok(msg{"op": "cancel", "transfer": "c1"})
	if _, err := feed.Write(make([]byte, 1024)); err != nil {
		t.Fatal(err)
	}
	_, end := h.events("c1")
	if end["event"] != "failed" || end["code"] != CodeCancelled || end["error"] != "sshsession: cancelled" {
		t.Fatalf("cancel ended with %v", end)
	}
	if _, err := os.Lstat(remote); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial upload left behind: %v", err)
	}
	// A cancel for a finished transfer is harmless.
	h.ok(msg{"op": "cancel", "transfer": "c1"})
}
