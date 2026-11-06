// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package sshsession

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestReadText(t *testing.T) {
	h := newHarness(t, nil)
	writeFile(t, h.path("a.conf"), []byte("key = välue\n"))
	mt := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	if err := os.Chtimes(h.path("a.conf"), mt, mt); err != nil {
		t.Fatal(err)
	}
	res := h.ok(msg{"op": "read", "path": h.path("a.conf")})
	if res["content"] != "key = välue\n" || res["mtime"] != float64(mt.UnixMilli()) || res["size"] != float64(len("key = välue\n")) {
		t.Fatalf("read = %v", res)
	}
	writeFile(t, h.path("bin"), []byte{'a', 0, 'b'})
	h.fails(msg{"op": "read", "path": h.path("bin")}, CodeBinary)
	writeFile(t, h.path("latin1"), []byte{'c', 'a', 'f', 0xe9})
	h.fails(msg{"op": "read", "path": h.path("latin1")}, CodeBinary)
	writeFile(t, h.path("big"), []byte(strings.Repeat("x", maxEditBytes+1)))
	h.fails(msg{"op": "read", "path": h.path("big")}, CodeTooLarge)
	writeFile(t, h.path("edge"), []byte(strings.Repeat("y", maxEditBytes)))
	if res := h.ok(msg{"op": "read", "path": h.path("edge")}); len(res["content"].(string)) != maxEditBytes {
		t.Fatal("a file of exactly 2 MB did not open")
	}
	h.fails(msg{"op": "read", "path": h.root}, CodeIsDir)
}

func TestWriteText(t *testing.T) {
	h := newHarness(t, nil)
	p := h.path("app.conf")
	writeFile(t, p, []byte("old\n"))
	if err := os.Chmod(p, 0o640); err != nil {
		t.Fatal(err)
	}
	// A sub-second mtime on disk reads back as whole seconds and must not count as a change.
	mt := time.Date(2026, 5, 6, 7, 8, 9, 700_000_000, time.UTC)
	if err := os.Chtimes(p, mt, mt); err != nil {
		t.Fatal(err)
	}
	read := h.ok(msg{"op": "read", "path": p})
	res := h.ok(msg{"op": "write", "path": p, "content": "new\n", "mtime": read["mtime"]})
	if b, _ := os.ReadFile(p); string(b) != "new\n" {
		t.Fatalf("content = %q", b)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v not kept", fi.Mode().Perm())
	}
	if res["mtime"] == nil {
		t.Fatalf("write reply = %v", res)
	}
	// The returned mtime is good for the next save.
	h.ok(msg{"op": "write", "path": p, "content": "newer\n", "mtime": res["mtime"]})

	// Someone else changes the file: the stale mtime is refused, force writes anyway.
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}
	h.fails(msg{"op": "write", "path": p, "content": "mine\n", "mtime": res["mtime"]}, CodeChanged)
	if b, _ := os.ReadFile(p); string(b) != "newer\n" {
		t.Fatalf("a refused write changed the file: %q", b)
	}
	h.ok(msg{"op": "write", "path": p, "content": "mine\n", "mtime": res["mtime"], "force": true})
	if b, _ := os.ReadFile(p); string(b) != "mine\n" {
		t.Fatalf("force write = %q", b)
	}

	// New files take mtime 0; a taken name is exists, a file deleted since the read is changed.
	h.ok(msg{"op": "write", "path": h.path("fresh.txt"), "content": "hi", "mtime": 0})
	if b, _ := os.ReadFile(h.path("fresh.txt")); string(b) != "hi" {
		t.Fatalf("new file = %q", b)
	}
	h.fails(msg{"op": "write", "path": h.path("fresh.txt"), "content": "x", "mtime": 0}, CodeExists)
	h.fails(msg{"op": "write", "path": h.path("gone.txt"), "content": "x", "mtime": 1000}, CodeChanged)
	h.fails(msg{"op": "write", "path": h.root, "content": "x", "mtime": 1000, "force": true}, CodeIsDir)

	entries, _ := os.ReadDir(h.root)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".lgrass-") {
			t.Fatalf("temp file left: %s", e.Name())
		}
	}
}

func TestWriteLargeContent(t *testing.T) {
	h := newHarness(t, nil)
	p := h.path("big.txt")
	writeFile(t, p, []byte("x"))
	read := h.ok(msg{"op": "read", "path": p})
	content := strings.Repeat("\"quoted\"\tline\n", maxEditBytes/16)
	h.ok(msg{"op": "write", "path": p, "content": content, "mtime": read["mtime"]})
	if b, _ := os.ReadFile(p); string(b) != content {
		t.Fatal("large content differs after write")
	}
}
