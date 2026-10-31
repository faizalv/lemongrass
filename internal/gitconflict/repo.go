// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package gitconflict

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Repo is a git work tree, with paths relative to its top level as git and the app's conflict code name them.
type Repo struct{ Top string }

func OpenRepo(dir string) (Repo, error) {
	out, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repo{}, err
	}
	return Repo{Top: strings.TrimSpace(string(out))}, nil
}

func git(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_LITERAL_PATHSPECS=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gitconflict: git %s: %s", args[0], strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// Unmerged lists the paths git still holds in conflict. A path stays here until the human stages it, whatever the model wrote to it.
func (r Repo) Unmerged() ([]string, error) {
	out, err := git(r.Top, "diff", "--name-only", "-z", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// A missing file hashes to "-", so deleting a conflicted file counts as a change.
func (r Repo) fileHash(path string) (string, error) {
	data, err := os.ReadFile(filepath.Join(r.Top, path))
	if errors.Is(err, fs.ErrNotExist) {
		return "-", nil
	}
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Baseline records what every conflicted file holds now, as the JSON the store keeps on the conflict thread.
func (r Repo) Baseline() (string, []string, error) {
	paths, err := r.Unmerged()
	if err != nil {
		return "", nil, err
	}
	hashes := make(map[string]string, len(paths))
	for _, p := range paths {
		if hashes[p], err = r.fileHash(p); err != nil {
			return "", nil, err
		}
	}
	data, err := json.Marshal(hashes)
	return string(data), paths, err
}

// Changed lists the paths still in conflict whose working file differs from baseline. A path the baseline does not know counts as changed.
func (r Repo) Changed(baseline string) ([]string, error) {
	hashes := map[string]string{}
	if baseline != "" {
		if err := json.Unmarshal([]byte(baseline), &hashes); err != nil {
			return nil, fmt.Errorf("gitconflict: reading the hand-off baseline: %w", err)
		}
	}
	paths, err := r.Unmerged()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range paths {
		h, err := r.fileHash(p)
		if err != nil {
			return nil, err
		}
		if before, ok := hashes[p]; !ok || before != h {
			out = append(out, p)
		}
	}
	return out, nil
}

// Only a conflict opener of git's default marker size or longer counts. It is a hint for the model's report, not a parse; the app reads the blocks itself.
var markerOpen = regexp.MustCompile(`(?m)^<{7,}( |\r?$)`)

func (r Repo) HoldsMarkers(path string) bool {
	data, err := os.ReadFile(filepath.Join(r.Top, path))
	return err == nil && markerOpen.Match(data)
}

// Rel turns a path given on the command line, relative to dir or absolute, into a path relative to the top level, refusing anything outside it.
func (r Repo) Rel(dir, arg string) (string, error) {
	p := arg
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	top := r.Top
	if real, err := filepath.EvalSymlinks(top); err == nil {
		top = real
	}
	parent := filepath.Dir(p)
	if real, err := filepath.EvalSymlinks(parent); err == nil {
		p = filepath.Join(real, filepath.Base(p))
	}
	rel, err := filepath.Rel(top, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("gitconflict: %s is outside the repository", arg)
	}
	return filepath.ToSlash(rel), nil
}

// ParseLines reads "N" or "N-M", 1-based and inclusive.
func ParseLines(s string) (int, int, error) {
	a, b, isRange := strings.Cut(s, "-")
	start, err := strconv.Atoi(a)
	end := start
	if err == nil && isRange {
		end, err = strconv.Atoi(b)
	}
	if err != nil || start < 1 || end < start {
		return 0, 0, fmt.Errorf("gitconflict: lines %q are not N or N-M with N at least 1 and M at least N", s)
	}
	return start, end, nil
}

// Quote returns lines start to end of the working file, refusing a range past its end.
func (r Repo) Quote(path string, start, end int) (string, error) {
	data, err := os.ReadFile(filepath.Join(r.Top, path))
	if err != nil {
		return "", fmt.Errorf("gitconflict: reading %s: %w", path, err)
	}
	text := strings.TrimSuffix(string(data), "\n")
	lines := strings.Split(text, "\n")
	if text == "" {
		lines = nil
	}
	if end > len(lines) {
		return "", fmt.Errorf("gitconflict: %s has %d lines, so lines %d-%d do not exist", path, len(lines), start, end)
	}
	return strings.Join(lines[start-1:end], "\n"), nil
}
