// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package sshsession

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/sftp"
)

type msg map[string]any

type harness struct {
	t       *testing.T
	root    string
	in      *io.PipeWriter
	msgs    chan msg
	backlog []msg
	served  chan error
	dials   atomic.Int32
	nextID  int
	servers chan net.Conn
}

// newHarness runs a session whose transport is an in-process SFTP server rooted at a temp folder.
// fail makes the n-th dial fail, for the notLoggedIn cases.
func newHarness(t *testing.T, fail func(n int32) error) *harness {
	t.Helper()
	h := blankHarness(t)
	h.start(func(ctx context.Context) (io.ReadWriteCloser, error) {
		n := h.dials.Add(1)
		if fail != nil {
			if err := fail(n); err != nil {
				return nil, err
			}
		}
		cli, srv := net.Pipe()
		server, err := sftp.NewServer(srv, sftp.WithServerWorkingDirectory(h.root))
		if err != nil {
			return nil, err
		}
		h.servers <- srv
		go server.Serve()
		return cli, nil
	})
	return h
}

// newHarnessWith runs a session over an arbitrary transport.
func newHarnessWith(t *testing.T, dial Dialer) *harness {
	t.Helper()
	h := blankHarness(t)
	h.start(dial)
	return h
}

func blankHarness(t *testing.T) *harness {
	return &harness{t: t, root: t.TempDir(), msgs: make(chan msg, 1024), served: make(chan error, 1),
		servers: make(chan net.Conn, 16)}
}

func (h *harness) start(dial Dialer) {
	t := h.t
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	h.in = inW
	go func() {
		h.served <- New(dial).Serve(inR, outW)
		outW.Close()
	}()
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var m msg
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				t.Errorf("bad output line %q: %v", sc.Text(), err)
				continue
			}
			h.msgs <- m
		}
		close(h.msgs)
	}()
	t.Cleanup(func() {
		inW.Close()
		select {
		case <-h.served:
		case <-time.After(10 * time.Second):
			t.Error("session did not exit after stdin closed")
		}
	})
}

func (h *harness) send(req msg) string {
	h.t.Helper()
	h.nextID++
	id := fmt.Sprintf("r%d", h.nextID)
	req["id"] = id
	b, _ := json.Marshal(req)
	if _, err := h.in.Write(append(b, '\n')); err != nil {
		h.t.Fatalf("write request: %v", err)
	}
	return id
}

// next returns the first message matching pred, keeping the others for later calls.
func (h *harness) next(pred func(msg) bool) msg {
	h.t.Helper()
	for i, m := range h.backlog {
		if pred(m) {
			h.backlog = append(h.backlog[:i], h.backlog[i+1:]...)
			return m
		}
	}
	timeout := time.After(10 * time.Second)
	for {
		select {
		case m, ok := <-h.msgs:
			if !ok {
				h.t.Fatal("output closed")
			}
			if pred(m) {
				return m
			}
			h.backlog = append(h.backlog, m)
		case <-timeout:
			h.t.Fatal("timed out waiting for a message")
		}
	}
}

func (h *harness) call(req msg) msg {
	h.t.Helper()
	id := h.send(req)
	return h.next(func(m msg) bool { return m["id"] == id })
}

func (h *harness) ok(req msg) msg {
	h.t.Helper()
	r := h.call(req)
	if r["ok"] != true {
		h.t.Fatalf("%v failed: %v", req["op"], r)
	}
	res, _ := r["result"].(map[string]any)
	return res
}

func (h *harness) fails(req msg, code string) msg {
	h.t.Helper()
	r := h.call(req)
	if r["ok"] != false || r["code"] != code {
		h.t.Fatalf("%v: want code %q, got %v", req["op"], code, r)
	}
	if e, _ := r["error"].(string); !strings.HasPrefix(e, "sshsession: ") {
		h.t.Fatalf("error %q lacks the package prefix", e)
	}
	return r
}

// events collects one transfer's progress events up to its done or failed event.
func (h *harness) events(transfer string) (progress []msg, end msg) {
	h.t.Helper()
	for {
		m := h.next(func(m msg) bool { return m["transfer"] == transfer && m["event"] != nil })
		if m["event"] == "progress" {
			progress = append(progress, m)
			continue
		}
		return progress, m
	}
}

func (h *harness) path(rel string) string { return filepath.Join(h.root, rel) }

func writeFile(t *testing.T, p string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHomeAndList(t *testing.T) {
	h := newHarness(t, nil)
	if got := h.ok(msg{"op": "home"})["path"]; got != h.root {
		t.Fatalf("home = %v, want %s", got, h.root)
	}
	writeFile(t, h.path("b.txt"), []byte("hello"))
	writeFile(t, h.path("Zdir/inner"), nil)
	writeFile(t, h.path("adir/inner"), nil)
	mtime := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(h.path("b.txt"), mtime, mtime); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(h.path("adir"), h.path("link-dir")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(h.path("missing"), h.path("broken")); err != nil {
			t.Fatal(err)
		}
	}
	res := h.ok(msg{"op": "list", "path": h.root})
	if res["path"] != h.root {
		t.Fatalf("list path = %v", res["path"])
	}
	byName := map[string]map[string]any{}
	var order []string
	for _, e := range res["entries"].([]any) {
		m := e.(map[string]any)
		byName[m["name"].(string)] = m
		order = append(order, m["name"].(string))
	}
	want := []string{"adir", "link-dir", "Zdir", "b.txt", "broken"}
	if runtime.GOOS == "windows" {
		want = []string{"adir", "Zdir", "b.txt"}
	}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", order, want)
	}
	f := byName["b.txt"]
	if f["dir"] != false || f["link"] != false || f["size"] != float64(5) {
		t.Fatalf("b.txt = %v", f)
	}
	if f["mode"] != "-rw-r--r--" && runtime.GOOS != "windows" {
		t.Fatalf("b.txt mode = %v", f["mode"])
	}
	if f["mtime"] != float64(mtime.UnixMilli()) {
		t.Fatalf("b.txt mtime = %v, want %d", f["mtime"], mtime.UnixMilli())
	}
	if d := byName["adir"]; d["dir"] != true || d["size"] != float64(0) || !strings.HasPrefix(d["mode"].(string), "d") {
		t.Fatalf("adir = %v", d)
	}
	if runtime.GOOS != "windows" {
		if l := byName["link-dir"]; l["dir"] != true || l["link"] != true || !strings.HasPrefix(l["mode"].(string), "l") {
			t.Fatalf("link-dir = %v", l)
		}
		if b := byName["broken"]; b["dir"] != false || b["link"] != true {
			t.Fatalf("broken = %v", b)
		}
	}
	// A relative path resolves against home.
	if got := h.ok(msg{"op": "list", "path": "adir"})["path"]; got != h.path("adir") {
		t.Fatalf("relative list path = %v", got)
	}
}

func TestMkdirRenameExists(t *testing.T) {
	h := newHarness(t, nil)
	h.ok(msg{"op": "mkdir", "path": h.path("new")})
	if fi, err := os.Stat(h.path("new")); err != nil || !fi.IsDir() {
		t.Fatalf("mkdir did not make a folder: %v", err)
	}
	h.fails(msg{"op": "mkdir", "path": h.path("new")}, CodeExists)

	writeFile(t, h.path("a.txt"), []byte("a"))
	writeFile(t, h.path("b.txt"), []byte("b"))
	h.ok(msg{"op": "rename", "from": h.path("a.txt"), "to": h.path("new/a.txt")})
	if _, err := os.Stat(h.path("new/a.txt")); err != nil {
		t.Fatalf("rename target missing: %v", err)
	}
	h.fails(msg{"op": "rename", "from": h.path("b.txt"), "to": h.path("new/a.txt")}, CodeExists)
	if b, _ := os.ReadFile(h.path("new/a.txt")); string(b) != "a" {
		t.Fatalf("rename overwrote the target: %q", b)
	}
}

func TestRemoveRecursive(t *testing.T) {
	h := newHarness(t, nil)
	writeFile(t, h.path("tree/a/b/c.txt"), []byte("c"))
	writeFile(t, h.path("tree/top.txt"), []byte("t"))
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "keep.txt"), []byte("k"))
	if runtime.GOOS != "windows" {
		if err := os.Symlink(outside, h.path("tree/a/out")); err != nil {
			t.Fatal(err)
		}
	}
	h.ok(msg{"op": "remove", "path": h.path("tree")})
	if _, err := os.Lstat(h.path("tree")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tree still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "keep.txt")); err != nil {
		t.Fatalf("remove followed a link out of the tree: %v", err)
	}
	writeFile(t, h.path("one.txt"), nil)
	h.ok(msg{"op": "remove", "path": h.path("one.txt")})
	if _, err := os.Lstat(h.path("one.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file still there: %v", err)
	}
	r := h.call(msg{"op": "remove", "path": "/"})
	if r["ok"] != false {
		t.Fatalf("remove / was not refused: %v", r)
	}
}

func TestUploadDownloadProgress(t *testing.T) {
	old := progressEvery
	progressEvery = 0
	t.Cleanup(func() { progressEvery = old })
	h := newHarness(t, nil)
	local := t.TempDir()
	data := bytes.Repeat([]byte("0123456789abcdef"), 256*1024) // 4 MiB
	writeFile(t, filepath.Join(local, "big.bin"), data)

	res := h.ok(msg{"op": "upload", "local": filepath.Join(local, "big.bin"), "remote": h.path("big.bin"), "transfer": "u1"})
	if res["transfer"] != "u1" || res["total"] != float64(len(data)) {
		t.Fatalf("upload reply = %v", res)
	}
	progress, end := h.events("u1")
	if end["event"] != "done" {
		t.Fatalf("upload ended with %v", end)
	}
	checkProgress(t, progress, len(data))
	if got, _ := os.ReadFile(h.path("big.bin")); !bytes.Equal(got, data) {
		t.Fatal("uploaded bytes differ")
	}

	target := filepath.Join(local, "copy.bin")
	h.ok(msg{"op": "download", "remote": h.path("big.bin"), "local": target, "transfer": "d1"})
	progress, end = h.events("d1")
	if end["event"] != "done" {
		t.Fatalf("download ended with %v", end)
	}
	checkProgress(t, progress, len(data))
	if got, _ := os.ReadFile(target); !bytes.Equal(got, data) {
		t.Fatal("downloaded bytes differ")
	}

	// Neither direction overwrites, and folders are refused before any transfer starts.
	h.fails(msg{"op": "upload", "local": filepath.Join(local, "big.bin"), "remote": h.path("big.bin"), "transfer": "u2"}, CodeExists)
	h.fails(msg{"op": "download", "remote": h.path("big.bin"), "local": target, "transfer": "d2"}, CodeExists)
	h.fails(msg{"op": "upload", "local": local, "remote": h.path("folder"), "transfer": "u3"}, CodeIsDir)
	h.fails(msg{"op": "download", "remote": h.root, "local": filepath.Join(local, "x"), "transfer": "d3"}, CodeIsDir)
	if r := h.call(msg{"op": "download", "remote": h.path("big.bin"), "local": "relative.bin", "transfer": "d4"}); r["ok"] != false {
		t.Fatalf("relative local path accepted: %v", r)
	}
}

func checkProgress(t *testing.T, progress []msg, total int) {
	t.Helper()
	if len(progress) < 2 {
		t.Fatalf("want several progress events, got %d", len(progress))
	}
	prev := -1.0
	for _, p := range progress {
		d := p["done"].(float64)
		if d < prev || p["total"] != float64(total) {
			t.Fatalf("bad progress %v after %v", p, prev)
		}
		prev = d
	}
	if prev != float64(total) {
		t.Fatalf("last progress %v, want %d", prev, total)
	}
}

func TestNotLoggedInThenRetry(t *testing.T) {
	h := newHarness(t, func(n int32) error {
		if n == 1 {
			return errors.New("no master")
		}
		return nil
	})
	h.fails(msg{"op": "list", "path": "/"}, CodeNotLoggedIn)
	h.ok(msg{"op": "home"})
	h.ok(msg{"op": "home"})
	if n := h.dials.Load(); n != 2 {
		t.Fatalf("dials = %d, want 2 (one failed, one kept)", n)
	}
}

func TestHandshakeFailureIsNotLoggedIn(t *testing.T) {
	var dials atomic.Int32
	inR, inW := io.Pipe()
	var out bytes.Buffer
	done := make(chan error)
	go func() {
		done <- New(func(ctx context.Context) (io.ReadWriteCloser, error) {
			dials.Add(1)
			cli, srv := net.Pipe()
			srv.Close() // ssh exited before sftp started
			return cli, nil
		}).Serve(inR, &out)
	}()
	fmt.Fprintln(inW, `{"id":1,"op":"home"}`)
	inW.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var r msg
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("reply %q: %v", out.String(), err)
	}
	if r["ok"] != false || r["code"] != CodeNotLoggedIn || r["id"] != float64(1) {
		t.Fatalf("reply = %v", r)
	}
}

func TestDroppedConnectionRedials(t *testing.T) {
	h := newHarness(t, nil)
	h.ok(msg{"op": "home"})
	(<-h.servers).Close()
	var last msg
	for i := 0; i < 3; i++ {
		last = h.call(msg{"op": "list", "path": h.root})
		if last["ok"] == true {
			break
		}
		if last["code"] != CodeNotLoggedIn {
			t.Fatalf("a dropped connection should read as notLoggedIn: %v", last)
		}
	}
	if last["ok"] != true || h.dials.Load() != 2 {
		t.Fatalf("no redial: %v, dials %d", last, h.dials.Load())
	}
}
