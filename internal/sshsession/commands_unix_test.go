// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build unix

package sshsession

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
)

func TestTouch(t *testing.T) {
	h := newHarness(t, nil)
	h.ok(msg{"op": "touch", "path": h.path("new.txt")})
	if fi, err := os.Stat(h.path("new.txt")); err != nil || fi.Size() != 0 || fi.IsDir() {
		t.Fatalf("touch made %v, %v", fi, err)
	}
	writeFile(t, h.path("full.txt"), []byte("keep"))
	h.fails(msg{"op": "touch", "path": h.path("full.txt")}, CodeExists)
	if b, _ := os.ReadFile(h.path("full.txt")); string(b) != "keep" {
		t.Fatalf("touch changed an existing file: %q", b)
	}
}

func TestMove(t *testing.T) {
	h := newHarness(t, nil)
	writeFile(t, h.path("a/one.txt"), []byte("1"))
	writeFile(t, h.path("b/keep.txt"), []byte("k"))
	h.ok(msg{"op": "mkdir", "path": h.path("dest")})
	h.ok(msg{"op": "move", "from": h.path("a"), "to": h.path("dest/a")})
	if b, _ := os.ReadFile(h.path("dest/a/one.txt")); string(b) != "1" {
		t.Fatalf("moved folder content = %q", b)
	}
	h.fails(msg{"op": "move", "from": h.path("b"), "to": h.path("dest/a")}, CodeExists)

	// A missing source fails over SFTP, falls back to mv, and replies with mv's own reason.
	r := h.call(msg{"op": "move", "from": h.path("missing"), "to": h.path("dest/missing")})
	if e, _ := r["error"].(string); r["ok"] != false || !strings.HasPrefix(e, "sshsession: mv: ") {
		t.Fatalf("missing source: %v", r)
	}

	// Paths with spaces, quotes and shell syntax reach mv as plain data.
	odd := h.path(`it's "odd" $(x) ;`)
	writeFile(t, odd, []byte("o"))
	h.ok(msg{"op": "move", "from": odd, "to": h.path("dest/odd")})
	if b, _ := os.ReadFile(h.path("dest/odd")); string(b) != "o" {
		t.Fatalf("odd name move = %q", b)
	}
	if r := h.call(msg{"op": "copy", "from": h.path("dest/odd"), "to": h.path("x\ny")}); r["ok"] != false {
		t.Fatalf("a path with a line break was accepted: %v", r)
	}
}

// SFTP rename cannot cross filesystems, so this move only succeeds through the mv fallback.
func TestMoveAcrossFilesystems(t *testing.T) {
	h := newHarness(t, nil)
	other, err := os.MkdirTemp("/dev/shm", "sshsession-")
	if err != nil {
		t.Skipf("no /dev/shm: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(other) })
	var a, b syscall.Stat_t
	if syscall.Stat(h.root, &a) != nil || syscall.Stat(other, &b) != nil || a.Dev == b.Dev {
		t.Skip("temp folder and /dev/shm are on the same filesystem")
	}
	writeFile(t, h.path("tree/f.txt"), []byte("f"))
	h.ok(msg{"op": "move", "from": h.path("tree"), "to": filepath.Join(other, "tree")})
	if b, _ := os.ReadFile(filepath.Join(other, "tree/f.txt")); string(b) != "f" {
		t.Fatalf("cross-filesystem move content = %q", b)
	}
	if _, err := os.Lstat(h.path("tree")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source left behind: %v", err)
	}
}

func TestCopy(t *testing.T) {
	h := newHarness(t, nil)
	writeFile(t, h.path("src/a/b.txt"), []byte("b"))
	writeFile(t, h.path("src/top.txt"), []byte("t"))
	if err := os.Chmod(h.path("src/top.txt"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.ok(msg{"op": "copy", "from": h.path("src"), "to": h.path("dup")})
	if b, _ := os.ReadFile(h.path("dup/a/b.txt")); string(b) != "b" {
		t.Fatalf("copied content = %q", b)
	}
	if fi, _ := os.Stat(h.path("dup/top.txt")); fi == nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("cp -a did not keep the mode: %v", fi)
	}
	if _, err := os.Stat(h.path("src/a/b.txt")); err != nil {
		t.Fatalf("copy removed the source: %v", err)
	}
	h.fails(msg{"op": "copy", "from": h.path("src/top.txt"), "to": h.path("dup/top.txt")}, CodeExists)
	h.ok(msg{"op": "copy", "from": h.path("src/top.txt"), "to": h.path("single.txt")})
}

func tarNames(t *testing.T, file string) []string {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("not gzip: %v", err)
	}
	tr := tar.NewReader(zr)
	var names []string
	for {
		hd, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("bad tar: %v", err)
		}
		names = append(names, strings.TrimSuffix(hd.Name, "/"))
	}
	sort.Strings(names)
	return names
}

func TestDownloadDir(t *testing.T) {
	old := progressEvery
	progressEvery = 0
	t.Cleanup(func() { progressEvery = old })
	h := newHarness(t, nil)
	writeFile(t, h.path("logs/app.log"), []byte(strings.Repeat("line\n", 50000)))
	writeFile(t, h.path("logs/sub/x.log"), []byte("x"))
	local := t.TempDir()
	target := filepath.Join(local, "logs.tar.gz")

	res := h.ok(msg{"op": "downloadDir", "remote": h.path("logs"), "local": target, "transfer": "z1"})
	if res["transfer"] != "z1" || res["total"] != float64(0) {
		t.Fatalf("reply = %v", res)
	}
	progress, end := h.events("z1")
	if end["event"] != "done" {
		t.Fatalf("ended with %v", end)
	}
	fi, _ := os.Stat(target)
	if len(progress) == 0 || progress[len(progress)-1]["done"] != float64(fi.Size()) || progress[0]["total"] != float64(0) {
		t.Fatalf("progress %v for a %d byte archive", progress, fi.Size())
	}
	want := "logs,logs/app.log,logs/sub,logs/sub/x.log"
	if got := strings.Join(tarNames(t, target), ","); got != want {
		t.Fatalf("archive holds %s, want %s", got, want)
	}

	h.fails(msg{"op": "downloadDir", "remote": h.path("logs"), "local": target, "transfer": "z2"}, CodeExists)
	writeFile(t, filepath.Join(local, "old.tar.gz"), []byte("old"))
	h.ok(msg{"op": "downloadDir", "remote": h.path("logs/sub"), "local": filepath.Join(local, "old.tar.gz"), "transfer": "z3", "replace": true})
	if _, end := h.events("z3"); end["event"] != "done" {
		t.Fatalf("replace ended with %v", end)
	}
	if got := strings.Join(tarNames(t, filepath.Join(local, "old.tar.gz")), ","); got != "sub,sub/x.log" {
		t.Fatalf("replaced archive holds %s", got)
	}
	r := h.call(msg{"op": "downloadDir", "remote": h.path("logs/app.log"), "local": filepath.Join(local, "f.tar.gz"), "transfer": "z4"})
	if r["ok"] != false || !strings.Contains(r["error"].(string), "not a folder") {
		t.Fatalf("file source: %v", r)
	}
	if r := h.call(msg{"op": "downloadDir", "remote": "/", "local": filepath.Join(local, "root.tar.gz"), "transfer": "z5"}); r["ok"] != false {
		t.Fatalf("archiving / was accepted: %v", r)
	}
	// No temp files are left beside the targets.
	entries, _ := os.ReadDir(local)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			t.Fatalf("leftover temp file %s", e.Name())
		}
	}
}

// A replacing download that is cancelled keeps the file it was meant to replace.
func TestDownloadDirCancelKeepsOld(t *testing.T) {
	old := progressEvery
	progressEvery = 0
	t.Cleanup(func() { progressEvery = old })
	slow := func(ctx context.Context, script string) (*exec.Cmd, error) {
		return exec.CommandContext(ctx, "sh", "-c", "head -c 65536 /dev/zero; exec sleep 30"), nil
	}
	h := newHarnessRunner(t, nil, slow)
	writeFile(t, h.path("logs/a"), nil)
	local := t.TempDir()
	target := filepath.Join(local, "logs.tar.gz")
	writeFile(t, target, []byte("old"))

	h.ok(msg{"op": "downloadDir", "remote": h.path("logs"), "local": target, "transfer": "c1", "replace": true})
	h.next(func(m msg) bool { return m["transfer"] == "c1" && m["event"] == "progress" })
	h.ok(msg{"op": "cancel", "transfer": "c1"})
	_, end := h.events("c1")
	if end["event"] != "failed" || end["code"] != CodeCancelled {
		t.Fatalf("cancel ended with %v", end)
	}
	if b, _ := os.ReadFile(target); string(b) != "old" {
		t.Fatalf("old file lost: %q", b)
	}
	if entries, _ := os.ReadDir(local); len(entries) != 1 {
		t.Fatalf("temp file left: %v", entries)
	}

	// A plain download with replace commits over the old file, and keeps it on failure too.
	writeFile(t, h.path("f.txt"), []byte("new"))
	h.ok(msg{"op": "download", "remote": h.path("f.txt"), "local": target, "transfer": "d1", "replace": true})
	if _, end := h.events("d1"); end["event"] != "done" {
		t.Fatalf("replace download ended with %v", end)
	}
	if b, _ := os.ReadFile(target); string(b) != "new" {
		t.Fatalf("replace download wrote %q", b)
	}
	h.fails(msg{"op": "download", "remote": h.path("f.txt"), "local": local, "transfer": "d2", "replace": true}, CodeIsDir)
}

// The remote command line, as a login shell would parse it, runs the script with the paths it
// reads from stdin.
func TestRunnerCommandThroughShell(t *testing.T) {
	dir := t.TempDir()
	from, to := filepath.Join(dir, `a b'"$x`), filepath.Join(dir, "c")
	writeFile(t, from, []byte("a"))
	args := runnerArgs("web-1", "/h/cm", false, mvScript)
	cmd := exec.Command("sh", "-c", args[len(args)-1])
	cmd.Stdin = strings.NewReader(from + "\n" + to + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if b, _ := os.ReadFile(to); string(b) != "a" {
		t.Fatalf("moved content = %q", b)
	}
}
