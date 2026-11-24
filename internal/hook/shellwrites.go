// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/internal/session"
)

// A shell command writes files no edit tool reports, so the edit log learns them from what changed while the call ran:
// git status inside the project, and a walk of biblio/, which is gitignored. A file another tab writes in the same
// window is logged for both sessions, which its readers (the task close's footprint, task ownership) accept.

// The git status a PostToolUse runs is bounded, so a huge or locked repository delays the agent by at most this.
const shellGitTimeout = 3 * time.Second

// Filesystems with coarse timestamps (FAT, HFS+) round a write right after the start down to before it.
const mtimeSlack = 2 * time.Second

func recordShellStart(store *session.Store, payload hookEvent) {
	if _, _, _, isShell := shellToolCommand(payload); !isShell || payload.SessionID == "" {
		return
	}
	store.StartShellCall(payload.SessionID, payload.ToolUseID, time.Now())
}

// logShellWrites runs at PostToolUse and PostToolUseFailure, since a failed command may still have written files.
func logShellWrites(store *session.Store, payload hookEvent, projectPath string) {
	if _, _, _, isShell := shellToolCommand(payload); !isShell || payload.SessionID == "" {
		return
	}
	started, ok, err := store.FinishShellCall(payload.SessionID, payload.ToolUseID)
	if err != nil || !ok {
		return
	}
	for _, path := range changedSince(projectPath, started.Add(-mtimeSlack)) {
		store.LogFileActivity(payload.SessionID, path)
	}
}

// changedSince lists the absolute paths of regular files in the project modified at or after since: git's changed
// and untracked files, then every file under biblio/. Deletions are not reported.
func changedSince(projectPath string, since time.Time) []string {
	seen := map[string]bool{}
	var out []string
	for _, path := range append(gitChangedSince(projectPath, since), biblioChangedSince(projectPath, since)...) {
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	return out
}

func gitChangedSince(projectPath string, since time.Time) []string {
	ctx, cancel := context.WithTimeout(context.Background(), shellGitTimeout)
	defer cancel()
	prefix, err := gitOutput(ctx, projectPath, "rev-parse", "--show-prefix")
	if err != nil {
		return nil
	}
	status, err := gitOutput(ctx, projectPath, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--", ".")
	if err != nil {
		return nil
	}
	return changedInStatus(projectPath, strings.TrimSpace(string(prefix)), string(status), since)
}

// changedInStatus reads `git status --porcelain=v1 -z` output, whose paths are relative to the repository root, so
// prefix (the project's own path inside the repository) is cut off before joining them to the project.
func changedInStatus(projectPath, prefix, status string, since time.Time) []string {
	var out []string
	fields := strings.Split(status, "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		code, rel := f[:2], strings.TrimPrefix(f[3:], prefix)
		if code[0] == 'R' || code[0] == 'C' {
			i++ // the old path follows a rename or copy
		}
		if code[0] == 'D' || code[1] == 'D' {
			continue
		}
		path := filepath.Join(projectPath, filepath.FromSlash(rel))
		if modifiedSince(path, since) {
			out = append(out, path)
		}
	}
	return out
}

// gitOutput runs git without its optional index lock, so a status refresh never collides with the agent's own git
// command in the same repository.
func gitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "-c", "core.quotePath=false"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// biblioChangedSince walks biblio/, following it when it is a symlink, and reports paths under the project's own
// biblio/ spelling. A folder that is its own git repository is skipped, as the biblio history skips it.
func biblioChangedSince(projectPath string, since time.Time) []string {
	biblio := filepath.Join(projectPath, "biblio")
	root, err := filepath.EvalSymlinks(biblio)
	if err != nil {
		return nil
	}
	var out []string
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || path != root && isOwnRepo(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !modifiedSince(path, since) {
			return nil
		}
		if rel, err := filepath.Rel(root, path); err == nil {
			out = append(out, filepath.Join(biblio, rel))
		}
		return nil
	})
	return out
}

func isOwnRepo(dir string) bool {
	_, err := os.Lstat(filepath.Join(dir, ".git"))
	return err == nil
}

func modifiedSince(path string, since time.Time) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && !info.ModTime().Before(since)
}
