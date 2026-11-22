// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package bibliohistory

import (
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func nestedGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	git, err := GitPath()
	if err != nil {
		t.Skip(err)
	}
	cmd := exec.Command(git, append(append(isolation(), "-C", dir), args...)...)
	cmd.Env = gitEnv(nil)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func trackedPaths(t *testing.T, r *Repo) []string {
	t.Helper()
	out, err := r.run(nil, nil, "ls-tree", "-r", "--name-only", "-z", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
}

func TestSnapshotSkipsEmptyNestedRepo(t *testing.T) {
	r, work := newTestRepo(t)
	writeFile(t, work, "laws/a.md", "a")
	writeFile(t, work, "scratchpad/task/probe/notes.txt", "probe")
	nestedGit(t, filepath.Join(work, "scratchpad/task/probe"), "init", "--quiet")

	if !mustSnapshot(t, r, nil) {
		t.Fatal("expected a commit")
	}
	if got := trackedPaths(t, r); !slices.Equal(got, []string{"laws/a.md"}) {
		t.Fatalf("tracked %v, want only laws/a.md", got)
	}
}

func TestSnapshotSkipsCommittedNestedRepo(t *testing.T) {
	r, work := newTestRepo(t)
	writeFile(t, work, "laws/a.md", "a")
	probe := filepath.Join(work, "scratchpad/task/probe")
	writeFile(t, work, "scratchpad/task/probe/notes.txt", "probe")
	nestedGit(t, probe, "init", "--quiet")
	nestedGit(t, probe, "add", "--all")
	nestedGit(t, probe, "commit", "--quiet", "-m", "one")

	mustSnapshot(t, r, nil)
	if got := trackedPaths(t, r); !slices.Equal(got, []string{"laws/a.md"}) {
		t.Fatalf("tracked %v, want only laws/a.md", got)
	}
}

func TestSnapshotDropsFolderThatBecameRepo(t *testing.T) {
	r, work := newTestRepo(t)
	writeFile(t, work, "laws/a.md", "a")
	writeFile(t, work, "scratchpad/task/probe/notes.txt", "probe")
	mustSnapshot(t, r, nil)

	nestedGit(t, filepath.Join(work, "scratchpad/task/probe"), "init", "--quiet")
	writeFile(t, work, "laws/b.md", "b")
	if !mustSnapshot(t, r, nil) {
		t.Fatal("expected a commit")
	}
	if got := trackedPaths(t, r); !slices.Equal(got, []string{"laws/a.md", "laws/b.md"}) {
		t.Fatalf("tracked %v, want laws/a.md and laws/b.md", got)
	}
}

func TestSnapshotExcludesNestedRepoLiterally(t *testing.T) {
	r, work := newTestRepo(t)
	writeFile(t, work, "x [1]*/notes.txt", "probe")
	nestedGit(t, filepath.Join(work, "x [1]*"), "init", "--quiet")
	writeFile(t, work, "x 1z/notes.txt", "kept")

	mustSnapshot(t, r, nil)
	if got := trackedPaths(t, r); !slices.Equal(got, []string{"x 1z/notes.txt"}) {
		t.Fatalf("tracked %v, want only x 1z/notes.txt", got)
	}
}

func TestInsideGitDir(t *testing.T) {
	root := filepath.Join("p", "biblio")
	cases := map[string]bool{
		filepath.Join(root, "scratchpad", "t", "probe", ".git"):                  true,
		filepath.Join(root, "scratchpad", "t", "probe", ".git", "objects", "ab"): true,
		filepath.Join(root, "scratchpad", "t", "probe", "notes.txt"):             false,
		filepath.Join(root, "scratchpad", "t", ".gitkeep"):                       false,
	}
	for path, want := range cases {
		if got := insideGitDir(root, path); got != want {
			t.Errorf("insideGitDir(%q) = %v, want %v", path, got, want)
		}
	}
}
