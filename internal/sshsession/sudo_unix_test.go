// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build unix

package sshsession

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sudoHarness runs a session whose stream is the real sudo-mode command run by the local sh, with
// a fake sudo first on PATH standing in for the server's.
func sudoHarness(t *testing.T, fakeSudo string, paths []string) *harness {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "sudo"), []byte("#!/bin/sh\n"+fakeSudo+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := newHarnessWith(t, func(ctx context.Context) (io.ReadWriteCloser, error) {
		cmd := exec.Command("sh", "-c", sudoCommand(paths))
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		return startPipe(cmd, true)
	})
	return h
}

func TestSudoModeCodes(t *testing.T) {
	server := filepath.Join(t.TempDir(), "sftp-server")
	if err := os.WriteFile(server, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "none")

	h := sudoHarness(t, `echo "sudo: a password is required" >&2; exit 1`, []string{missing, server})
	r := h.fails(msg{"op": "home"}, CodeSudoPassword)
	if r["error"] != "sshsession: sudo: a password is required" {
		t.Fatalf("error = %v", r["error"])
	}
	// The next request dials again, so the drawer's retry after a sudoers change works.
	h.fails(msg{"op": "list", "path": "/"}, CodeSudoPassword)

	h = sudoHarness(t, `echo "sudo: user alice is not in the sudoers file" >&2; exit 1`, []string{server})
	h.fails(msg{"op": "home"}, CodeSudoRefused)

	h = sudoHarness(t, `exit 99`, []string{missing})
	r = h.fails(msg{"op": "home"}, CodeNoSftpServer)
	if !strings.Contains(r["error"].(string), missing) {
		t.Fatalf("noSftpServer error should name the paths searched: %v", r["error"])
	}
}

// With a sudo that just runs its command, sudo mode reaches the machine's real sftp-server.
func TestSudoModeServes(t *testing.T) {
	var real string
	for _, p := range sftpServerPaths {
		if _, err := os.Stat(p); err == nil {
			real = p
			break
		}
	}
	if real == "" {
		t.Skip("no sftp-server on this machine")
	}
	missing := filepath.Join(t.TempDir(), "none")
	h := sudoHarness(t, `[ "$1" = -n ] || exit 3; shift; exec "$@"`, []string{missing, real})
	home := h.ok(msg{"op": "home"})["path"].(string)
	if home == "" {
		t.Fatal("empty home")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "x.txt"), []byte("x"))
	entries := h.ok(msg{"op": "list", "path": dir})["entries"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["name"] != "x.txt" {
		t.Fatalf("entries = %v", entries)
	}
}
