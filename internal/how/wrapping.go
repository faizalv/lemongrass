// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package how

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// now is the clock behind wrap file names, replaced in tests.
var now = time.Now

const wrapSkeleton = "# Wrap of %s, %s\n\nOne line per item this session raised that needed a home: what it is, then where it now lives.\n\n-\n"

// workWrapping is the one action that writes a file: each wrap gets its own timestamped file under the task's wraps/,
// so a long task keeps every pause as history, archived with the task.
func workWrapping(root string, args []string) (string, error) {
	slug, err := parseSlug(args, "lgrass how work wrapping <slug>")
	if err != nil {
		return "", err
	}
	if inspectWork(root, slug).archived {
		return archivedNotice(slug), nil
	}
	rel, err := createWrapFile(root, slug, now())
	if err != nil {
		return "", err
	}
	return fragment("work/wrapping.md", map[string]string{"path": rel}) + "\n", nil
}

func createWrapFile(root, slug string, at time.Time) (string, error) {
	dir := filepath.Join("biblio", "scratchpad", slug, "wraps")
	if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
		return "", fmt.Errorf("how: create %s: %w", dir, err)
	}
	stamp := at.Format("2006-01-02-1504")
	body := fmt.Sprintf(wrapSkeleton, slug, at.Format("2006-01-02 15:04"))
	for n := 1; ; n++ {
		name := stamp + ".md"
		if n > 1 {
			name = fmt.Sprintf("%s-%d.md", stamp, n)
		}
		rel := filepath.ToSlash(filepath.Join(dir, name))
		// O_EXCL keeps two wraps in the same minute from overwriting each other.
		file, err := os.OpenFile(filepath.Join(root, dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("how: create %s: %w", rel, err)
		}
		_, werr := file.WriteString(body)
		if cerr := file.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return "", fmt.Errorf("how: write %s: %w", rel, werr)
		}
		return rel, nil
	}
}
