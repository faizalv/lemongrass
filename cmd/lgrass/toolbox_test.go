// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func toolboxRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if _, err := biblioInit(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func srcFile(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func inventory(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "biblio", "toolbox", "inventory.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestToolboxAddCopiesFilesAndWritesTheLine(t *testing.T) {
	root := toolboxRoot(t)
	q := srcFile(t, "slow.sql", "select 1;")
	out, err := toolboxAdd(root, []string{"slow-queries", "--about", "Lists the slowest statements.", "--tags", "db,perfReview", q})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "1 of 40") {
		t.Errorf("out = %q", out)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "biblio/toolbox/slow-queries/slow.sql")); string(got) != "select 1;" {
		t.Errorf("copied = %q", got)
	}
	if inv := inventory(t, root); inv != "slow-queries [db,perfReview] Lists the slowest statements.\n" {
		t.Errorf("inventory = %q", inv)
	}
}

func TestToolboxRefusesBadInput(t *testing.T) {
	root := toolboxRoot(t)
	q := srcFile(t, "a.sh", "true")
	for name, args := range map[string][]string{
		"no about":       {"x", q},
		"no file":        {"x", "--about", "Does a thing."},
		"bad slug":       {"Bad_Slug", "--about", "Does a thing.", q},
		"two sentences":  {"x", "--about", "Does a thing.\nAnd more.", q},
		"no period":      {"x", "--about", "Does a thing", q},
		"hyphenated tag": {"x", "--about", "Does a thing.", "--tags", "perf-review", q},
		"drop on add":    {"x", "--about", "Does a thing.", "--drop", "a.sh", q},
		"directory":      {"x", "--about", "Does a thing.", t.TempDir()},
	} {
		if _, err := toolboxAdd(root, args); err == nil {
			t.Errorf("%s: add passed", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "biblio/toolbox/x")); err == nil {
		t.Error("a refused add left its folder behind")
	}
}

func TestToolboxRefusesAddWhenFull(t *testing.T) {
	root := toolboxRoot(t)
	q := srcFile(t, "a.sh", "true")
	for i := range toolboxLimit {
		if _, err := toolboxAdd(root, []string{fmt.Sprintf("item-%d", i), "--about", "Does a thing.", q}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := toolboxAdd(root, []string{"one-more", "--about", "Does a thing.", q})
	if err == nil || !strings.Contains(err.Error(), "its limit") {
		t.Fatalf("41st add: err %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "biblio/toolbox/one-more")); err == nil {
		t.Error("a refused add created its folder")
	}
}

func TestToolboxRefusesADuplicateOrAnUnlistedFolder(t *testing.T) {
	root := toolboxRoot(t)
	q := srcFile(t, "a.sh", "true")
	if _, err := toolboxAdd(root, []string{"dup", "--about", "Does a thing.", q}); err != nil {
		t.Fatal(err)
	}
	if _, err := toolboxAdd(root, []string{"dup", "--about", "Does a thing.", q}); err == nil {
		t.Error("second add of the same slug passed")
	}
	if err := os.Mkdir(filepath.Join(root, "biblio/toolbox/stray"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := toolboxAdd(root, []string{"stray", "--about", "Does a thing.", q}); err == nil {
		t.Error("add over an unlisted folder passed")
	}
}

func TestToolboxUpdateAndRemove(t *testing.T) {
	root := toolboxRoot(t)
	a := srcFile(t, "a.sh", "old")
	b := srcFile(t, "b.sh", "b")
	if _, err := toolboxAdd(root, []string{"tool", "--about", "Does a thing.", a, b}); err != nil {
		t.Fatal(err)
	}
	newA := srcFile(t, "a.sh", "new")
	if _, err := toolboxUpdate(root, []string{"tool", newA, "--drop", "b.sh", "--about", "Does it better.", "--tags", "ops"}); err != nil {
		t.Fatal(err)
	}
	item := filepath.Join(root, "biblio/toolbox/tool")
	if got, _ := os.ReadFile(filepath.Join(item, "a.sh")); string(got) != "new" {
		t.Errorf("a.sh = %q", got)
	}
	if _, err := os.Stat(filepath.Join(item, "b.sh")); err == nil {
		t.Error("b.sh was not dropped")
	}
	if inv := inventory(t, root); inv != "tool [ops] Does it better.\n" {
		t.Errorf("inventory = %q", inv)
	}
	if _, err := toolboxUpdate(root, []string{"tool", "--drop", "a.sh"}); err == nil {
		t.Error("dropping the last file passed")
	}
	if _, err := toolboxUpdate(root, []string{"tool"}); err == nil {
		t.Error("update with nothing to change passed")
	}
	if _, err := toolboxUpdate(root, []string{"missing", "--about", "Does a thing."}); err == nil {
		t.Error("update of a missing item passed")
	}
	if _, err := toolboxRemove(root, []string{"tool"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(item); err == nil {
		t.Error("remove left the folder")
	}
	if inv := inventory(t, root); inv != "" {
		t.Errorf("inventory after remove = %q", inv)
	}
}

func TestToolboxListAndMissingBiblio(t *testing.T) {
	root := toolboxRoot(t)
	if out, _ := toolboxList(root, nil); !strings.Contains(out, "empty") {
		t.Errorf("empty list = %q", out)
	}
	q := srcFile(t, "a.sh", "true")
	toolboxAdd(root, []string{"zeta", "--about", "Second.", q})
	toolboxAdd(root, []string{"alpha", "--about", "First.", q})
	out, _ := toolboxList(root, nil)
	if !strings.Contains(out, "2 of 40") || strings.Index(out, "alpha") > strings.Index(out, "zeta") {
		t.Errorf("list = %q", out)
	}
	if _, err := toolboxList(t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), "lgrass biblio init") {
		t.Errorf("no biblio: err %v", err)
	}
}
