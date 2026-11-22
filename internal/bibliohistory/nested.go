// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package bibliohistory

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// nestedRepos lists, relative to root and with forward slashes, every folder below root that holds a .git entry, a
// folder or a file. It does not descend into them, since their content belongs to their own repository.
func nestedRepos(root string) []string {
	var found []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" {
			return filepath.SkipDir
		}
		if p == root {
			return nil
		}
		if _, err := os.Lstat(filepath.Join(p, ".git")); err == nil {
			if rel, err := filepath.Rel(root, p); err == nil {
				found = append(found, filepath.ToSlash(rel))
			}
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

// excludeNested keeps nested repositories out of the history. git add refuses an embedded repository with no commit
// checked out, which would stop every snapshot, and records one with commits as a bare commit id with none of its
// files. Tasks may legitimately keep a real repository under biblio, so they are excluded rather than forbidden.
func (r *Repo) excludeNested() error {
	nested := nestedRepos(r.workTree)
	var b strings.Builder
	for _, rel := range nested {
		b.WriteString("/" + escapeIgnore(rel) + "/\n")
	}
	info := filepath.Join(r.gitDir, "info")
	if err := os.MkdirAll(info, 0o700); err != nil {
		return fmt.Errorf("bibliohistory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(info, "exclude"), []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("bibliohistory: %w", err)
	}
	if len(nested) == 0 {
		return nil
	}
	// An exclude rule does not apply to paths already in the index, such as a folder committed before a repository
	// was created inside it, so those are dropped from the index too.
	args := []string{"rm", "-r", "--cached", "--quiet", "--ignore-unmatch", "--"}
	for _, rel := range nested {
		args = append(args, ":(literal)"+rel)
	}
	_, err := r.run(nil, nil, args...)
	return err
}

// escapeIgnore makes a path match itself literally as a gitignore pattern.
func escapeIgnore(rel string) string {
	var b strings.Builder
	for _, c := range rel {
		switch c {
		case '\\', '*', '?', '[', ']', ' ', '!', '#':
			b.WriteRune('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}
