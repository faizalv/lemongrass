// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func writeFileAt(t *testing.T, path string, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func gitRepo(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("biblio/\n"), 0o644)
}

func TestChangedInStatusCutsThePrefixAndSkipsRenameSourcesAndDeletions(t *testing.T) {
	project := t.TempDir()
	now := time.Now()
	writeFileAt(t, filepath.Join(project, "new.go"), now)
	writeFileAt(t, filepath.Join(project, "moved.go"), now)
	writeFileAt(t, filepath.Join(project, "old.go"), now.Add(-time.Hour))
	status := "?? app/new.go\x00R  app/moved.go\x00app/source.go\x00 D app/gone.go\x00 M app/old.go\x00"
	got := changedInStatus(project, "app/", status, now.Add(-time.Second))
	want := []string{filepath.Join(project, "new.go"), filepath.Join(project, "moved.go")}
	if !slices.Equal(got, want) {
		t.Errorf("changedInStatus = %v, want %v", got, want)
	}
}

func TestBiblioWalkSkipsOldFilesAndNestedRepositories(t *testing.T) {
	project := t.TempDir()
	now := time.Now()
	fresh := filepath.Join(project, "biblio", "scratchpad", "demo", "prd.md")
	writeFileAt(t, fresh, now)
	writeFileAt(t, filepath.Join(project, "biblio", "books", "old.md"), now.Add(-time.Hour))
	writeFileAt(t, filepath.Join(project, "biblio", "toolbox", "own", ".git", "HEAD"), now)
	writeFileAt(t, filepath.Join(project, "biblio", "toolbox", "own", "inside.md"), now)
	got := biblioChangedSince(project, now.Add(-time.Second))
	if !slices.Equal(got, []string{fresh}) {
		t.Errorf("biblioChangedSince = %v, want only %s", got, fresh)
	}
}

func TestAShellEditOfATaskFileMakesTheTabOwnTheTask(t *testing.T) {
	store := openHookTestStore(t)
	project := projectWithBiblio(t)
	gitRepo(t, project)
	past := time.Now().Add(-time.Hour)
	prd := filepath.Join(project, "biblio", "scratchpad", "demo", "prd.md")
	writeFileAt(t, prd, past)
	writeFileAt(t, filepath.Join(project, "stale.go"), past)

	pre := preTool("tab-1", "Bash", map[string]string{"command": "sed -i s/a/b/ biblio/scratchpad/demo/prd.md"})
	pre.ToolUseID = "call-1"
	pre.Cwd = project
	store.RecordTabSession("tab-1", pre.SessionID)
	if result := hookPreToolUse(store, pre, project); result.PermissionDecision == "deny" {
		t.Fatalf("the shell call was denied: %+v", result)
	}

	writeFileAt(t, prd, time.Now())
	writeFileAt(t, filepath.Join(project, "made.go"), time.Now())
	hookPostToolUse(store, pre, project)

	owners, err := store.TaskOwners(project)
	if err != nil || owners["tab-1"] != "demo" {
		t.Errorf("TaskOwners = %v, %v; want tab-1 owning demo", owners, err)
	}
	touches, _ := store.FileActivitySince(past)
	var logged []string
	for _, touch := range touches {
		logged = append(logged, touch.FilePath)
	}
	if !slices.Contains(logged, filepath.Join(project, "made.go")) || slices.Contains(logged, filepath.Join(project, "stale.go")) {
		t.Errorf("logged %v, want made.go and not stale.go", logged)
	}
}

func TestAFailedShellCallStillLogsWhatItWrote(t *testing.T) {
	store := openHookTestStore(t)
	project := projectWithBiblio(t)
	pre := preTool("tab-2", "Bash", map[string]string{"command": "false"})
	pre.ToolUseID = "call-2"
	hookPreToolUse(store, pre, project)
	note := filepath.Join(project, "biblio", "scratchpad", "demo", "notes.md")
	writeFileAt(t, note, time.Now())
	logShellWrites(store, pre, project)
	touches, _ := store.FileActivitySince(time.Now().Add(-time.Minute))
	if len(touches) != 1 || touches[0].FilePath != note {
		t.Errorf("logged %v, want only %s", touches, note)
	}
}

func TestNonShellToolsRecordNoStart(t *testing.T) {
	store := openHookTestStore(t)
	project := projectWithBiblio(t)
	read := preTool("tab-3", "Read", map[string]string{"file_path": "x"})
	read.ToolUseID = "call-3"
	hookPreToolUse(store, read, project)
	if _, ok, _ := store.FinishShellCall(read.SessionID, "call-3"); ok {
		t.Error("a Read call recorded a shell start")
	}
}
