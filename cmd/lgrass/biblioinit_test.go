// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestBiblioInitCreatesLayoutAndIgnoresIt(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err := biblioInit(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"biblio/scratchpad/", "biblio/laws/", "biblio/handover/", "biblio/books/", "biblio/books/toc.md", "biblio/laws/summary.md", "biblio/handover/whiteboard.md", "biblio/ in .gitignore"} {
		if !slices.Contains(created, want) {
			t.Errorf("created %v lacks %s", created, want)
		}
	}
	ignore, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if string(ignore) != "node_modules\nbiblio/\n" {
		t.Errorf(".gitignore = %q", ignore)
	}
}

func TestBiblioInitKeepsExistingContent(t *testing.T) {
	root := t.TempDir()
	toc := filepath.Join(root, "biblio", "books", "toc.md")
	if err := os.MkdirAll(filepath.Dir(toc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(toc, []byte("some_book [tag]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/biblio/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err := biblioInit(root)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(created, "biblio/books/toc.md") || slices.Contains(created, "biblio/ in .gitignore") {
		t.Errorf("created %v touched existing files", created)
	}
	if data, _ := os.ReadFile(toc); string(data) != "some_book [tag]\n" {
		t.Errorf("toc.md = %q", data)
	}
	again, err := biblioInit(root)
	if err != nil || len(again) != 0 {
		t.Errorf("second run created %v, err %v", again, err)
	}
}
