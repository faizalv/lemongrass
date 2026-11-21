// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"os"
	"path/filepath"
	"strings"
)

// TaskOwners maps each tab to the scratchpad task of its session's latest edit under the scratchpad, for the project
// rooted at root. Only that latest edit counts: when its task is archived or removed, or the file sits loose in the
// scratchpad, the tab owns nothing, so a closed task never hands the tab an older one. Ownership is read from the edit
// log alone: no model declares it.
func (s *Store) TaskOwners(root string) (map[string]string, error) {
	rows, err := s.db.Query(`
		SELECT ts.tab_id, fa.file_path FROM lg_tab_sessions ts
		JOIN file_activity fa ON fa.project_id = ts.project_id AND fa.session_id = ts.session_id
		WHERE ts.project_id = ? AND fa.file_path LIKE '%scratchpad%'
		ORDER BY fa.touched_at DESC
	`, s.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scratchpads := []string{filepath.Join(root, "biblio", "scratchpad")}
	if r, err := filepath.EvalSymlinks(scratchpads[0]); err == nil && r != scratchpads[0] {
		scratchpads = append(scratchpads, r)
	}
	owners := map[string]string{}
	decided := map[string]bool{}
	for rows.Next() {
		var tabID, path string
		if err := rows.Scan(&tabID, &path); err != nil {
			return nil, err
		}
		if decided[tabID] {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		slug, under := taskSlug(filepath.Clean(path), scratchpads)
		if !under {
			continue
		}
		decided[tabID] = true
		if slug != "" {
			owners[tabID] = slug
		}
	}
	return owners, rows.Err()
}

// The task a path sits in, and whether the path is under a scratchpad at all. The task is the first directory below the
// scratchpad while it still exists and is not the archive; a path under the scratchpad that names no live task gives
// an empty slug.
func taskSlug(path string, scratchpads []string) (string, bool) {
	for _, dir := range scratchpads {
		rel, err := filepath.Rel(dir, path)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 2 || parts[0] == "archive" {
			return "", true
		}
		if info, err := os.Stat(filepath.Join(dir, parts[0])); err != nil || !info.IsDir() {
			return "", true
		}
		return parts[0], true
	}
	return "", false
}
