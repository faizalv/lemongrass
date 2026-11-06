// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build unix

package sshsession

import (
	"os"
	"syscall"
	"testing"
)

func inode(t *testing.T, p string) uint64 {
	t.Helper()
	var st syscall.Stat_t
	if err := syscall.Lstat(p, &st); err != nil {
		t.Fatal(err)
	}
	return uint64(st.Ino)
}

// A normal save replaces the file through a rename, so readers never see it half written.
func TestWriteReplacesAtomically(t *testing.T) {
	h := newHarness(t, nil)
	p := h.path("a.txt")
	writeFile(t, p, []byte("old"))
	before := inode(t, p)
	read := h.ok(msg{"op": "read", "path": p})
	h.ok(msg{"op": "write", "path": p, "content": "new", "mtime": read["mtime"]})
	if inode(t, p) == before {
		t.Fatal("save wrote in place in a writable folder")
	}
}

func TestWriteThroughSymlink(t *testing.T) {
	h := newHarness(t, nil)
	writeFile(t, h.path("real.conf"), []byte("old"))
	if err := os.Symlink(h.path("real.conf"), h.path("link.conf")); err != nil {
		t.Fatal(err)
	}
	read := h.ok(msg{"op": "read", "path": h.path("link.conf")})
	if read["content"] != "old" {
		t.Fatalf("read through link = %v", read)
	}
	h.ok(msg{"op": "write", "path": h.path("link.conf"), "content": "new", "mtime": read["mtime"]})
	if fi, err := os.Lstat(h.path("link.conf")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was replaced by a file: %v %v", fi, err)
	}
	if b, _ := os.ReadFile(h.path("real.conf")); string(b) != "new" {
		t.Fatalf("link target = %q", b)
	}
}

// In a folder the user cannot write, the temp file cannot be made, so the save writes in place.
func TestWriteInPlaceInReadOnlyFolder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes any folder")
	}
	h := newHarness(t, nil)
	writeFile(t, h.path("ro/app.conf"), []byte("old"))
	before := inode(t, h.path("ro/app.conf"))
	if err := os.Chmod(h.path("ro"), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(h.path("ro"), 0o755) })
	read := h.ok(msg{"op": "read", "path": h.path("ro/app.conf")})
	h.ok(msg{"op": "write", "path": h.path("ro/app.conf"), "content": "new", "mtime": read["mtime"]})
	if b, _ := os.ReadFile(h.path("ro/app.conf")); string(b) != "new" {
		t.Fatalf("content = %q", b)
	}
	if inode(t, h.path("ro/app.conf")) != before {
		t.Fatal("expected an in-place write")
	}
}
