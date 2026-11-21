// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestTaskOwnersTakesTheLatestLiveTask(t *testing.T) {
	store := openTestStore(t)
	root := t.TempDir()
	pad := filepath.Join(root, "biblio", "scratchpad")
	for _, dir := range []string{"older", "newer", "archive/done"} {
		if err := os.MkdirAll(filepath.Join(pad, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustRecord := func(tab, session string) {
		if err := store.RecordTabSession(tab, session); err != nil {
			t.Fatal(err)
		}
	}
	touch := func(session, path string) {
		t.Helper()
		if err := store.LogFileActivity(session, path); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	mustRecord("tab-a", "sess-a")
	mustRecord("tab-b", "sess-b")
	mustRecord("tab-c", "sess-c")
	mustRecord("tab-d", "sess-d")

	touch("sess-a", filepath.Join(pad, "older", "prd.md"))
	touch("sess-a", filepath.Join(pad, "newer", "activities.md"))
	touch("sess-a", filepath.Join(root, "main.go"))
	// An archived or deleted task owns nothing, so the tab keeps the task it worked before.
	touch("sess-b", filepath.Join(pad, "older", "notes.md"))
	touch("sess-b", filepath.Join(pad, "archive", "done", "prd.md"))
	touch("sess-b", filepath.Join(pad, "removed", "prd.md"))
	// A relative path is read against the root; a loose scratchpad file is no task.
	touch("sess-c", "biblio/scratchpad/newer/prd.md")
	touch("sess-c", filepath.Join(pad, "loose.md"))
	touch("sess-d", filepath.Join(root, "other.go"))

	owners, err := store.TaskOwners(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"tab-a": "newer", "tab-b": "older", "tab-c": "newer"}
	if !reflect.DeepEqual(owners, want) {
		t.Fatalf("owners = %v, want %v", owners, want)
	}
}

func TestTabSessionsAllProjectsSpansProjects(t *testing.T) {
	store := openTestStore(t)
	other, err := Open(DBPath(), "otherproj")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := store.RecordTabSession("tab-a", "sess-a"); err != nil {
		t.Fatal(err)
	}
	if err := other.RecordTabSession("tab-b", "sess-b"); err != nil {
		t.Fatal(err)
	}
	all, err := store.TabSessionsAllProjects()
	if err != nil {
		t.Fatal(err)
	}
	if all["tab-a"] != "sess-a" || all["tab-b"] != "sess-b" {
		t.Fatalf("all = %v", all)
	}
}
