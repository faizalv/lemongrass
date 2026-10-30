// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package how

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func taskHandover(root string, args []string) (string, error) {
	slug, err := parseSlug(args, "lgrass how task handover <slug>")
	if err != nil {
		return "", err
	}
	scratchpad := filepath.Join(root, "biblio", "scratchpad")
	if info, err := os.Stat(filepath.Join(scratchpad, slug)); err != nil || !info.IsDir() {
		return "", fmt.Errorf("how: no active task directory biblio/scratchpad/%s", slug)
	}
	vars := map[string]string{"slug": slug}
	prd := filepath.Join(scratchpad, slug, "prd.md")
	if _, err := os.Stat(prd); err != nil {
		return fragment("task/handover/noprd.md", vars) + "\n", nil
	}
	plans, _, open := readPRD(prd)
	if plans > 0 && len(open) == 0 {
		return fragment("task/handover/finished.md", vars) + "\n", nil
	}

	var b strings.Builder
	if handover := findHandover(root, slug); handover != "" {
		vars["handover"] = handover
		vars["row"] = "The row for `" + slug + "` stays, renumbered only if its priority changed."
		if !onWhiteboard(root, slug) {
			vars["row"] = "`" + slug + "` has no row in `biblio/handover/whiteboard.md`. Add `N. " + slug + " [tags] -> " + handover + "` at its priority position and renumber the rows below it."
		}
		fmt.Fprintf(&b, "Update the handover for %s: biblio/handover/%s exists, %d of %d Plans items open.\n\n", slug, handover, len(open), plans)
		b.WriteString(fragment("task/handover/rules.md", vars) + "\n\n## Steps\n\n" + fragment("task/handover/update.md", vars) + "\n")
		return b.String(), nil
	}
	fmt.Fprintf(&b, "New handover for %s: %d of %d Plans items open.\n\n", slug, len(open), plans)
	b.WriteString(fragment("task/handover/rules.md", vars) + "\n\n## Steps\n\n" + fragment("task/handover/new.md", vars) + "\n")
	return b.String(), nil
}

func onWhiteboard(root, slug string) bool {
	data, err := os.ReadFile(filepath.Join(root, "biblio", "handover", "whiteboard.md"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if m := whiteboardRow.FindStringSubmatch(strings.TrimSpace(line)); m != nil && m[1] == slug {
			return true
		}
	}
	return false
}
