// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package how

import (
	"fmt"
	"strings"
	"testing"
)

// cleanProject writes a biblio/ with n books and n handed-over tasks and no drift.
func cleanProject(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	var toc, board strings.Builder
	for i := 1; i <= n; i++ {
		book := fmt.Sprintf("book_%d", i)
		writeFile(t, root, "biblio/books/"+book+"/some_chapter.md", "x")
		fmt.Fprintf(&toc, "%s [tag,camelTag]\n", book)
		slug := fmt.Sprintf("task-%d", i)
		task(t, root, slug, "- [ ] open")
		writeFile(t, root, "biblio/scratchpad/"+slug+"/activities.md", "x")
		writeFile(t, root, "biblio/handover/"+slug+".md", "# h\n\nSee `biblio/scratchpad/"+slug+"/prd.md`.\n\n## Status\n\nx\n")
		fmt.Fprintf(&board, "%d. %s [tag] -> %s.md\n", i, slug, slug)
	}
	writeFile(t, root, "biblio/books/toc.md", toc.String())
	writeFile(t, root, "biblio/handover/whiteboard.md", board.String())
	return root
}

func TestKeeperSweepOutputDoesNotGrowWithProject(t *testing.T) {
	small := render(t, cleanProject(t, 3), "keeper", "sweep")
	large := render(t, cleanProject(t, 100), "keeper", "sweep")
	if small != large {
		t.Fatalf("clean sweep output differs between 3 and 100 books:\n%s\n---\n%s", small, large)
	}
	wantAll(t, small, "None found", "Memory pointers", "Git against PRD marks", "Books against code")
}

func TestKeeperSweepFindsDrift(t *testing.T) {
	root := cleanProject(t, 2)
	writeFile(t, root, "biblio/handover/whiteboard.md", "1. task-1 [tag] -> task-1.md\n3. gone [tag] -> gone.md\n4. old [tag] -> old.md\n")
	writeFile(t, root, "biblio/handover/archive/old.md", "x")
	writeFile(t, root, "biblio/handover/loose.md", "# loose\n")
	writeFile(t, root, "biblio/scratchpad/empty/scratch.txt", "x")
	task(t, root, "finished", "- [x] done")
	writeFile(t, root, "biblio/scratchpad/stray.md", "x")
	writeFile(t, root, "biblio/books/toc.md", "book_1 [tag,bad-tag]\nbook_9 [x]\nbook_2 [tag] a description\n")
	writeFile(t, root, "biblio/books/Bad Name/c.md", "x")
	writeFile(t, root, "biblio/books/book_1/[2026] Chapter.md", "x")
	out := render(t, root, "keeper", "sweep")
	wantAll(t, out,
		"row gone points at gone.md, which does not exist",
		"row old points at an archived handover",
		"not numbered 1, 2, 3",
		"task-2.md has no whiteboard row",
		"loose.md has no Status section",
		"loose.md points at no prd.md",
		"scratchpad/empty holds no prd.md",
		"scratchpad/finished has every Plans item done",
		"stray.md is a bare file",
		"bad-tag",
		"toc lists book_9",
		`toc line "book_2 [tag] a description"`,
		"book folder Bad Name is not a plain snake_case title",
		"book_1/[2026] Chapter.md is not a snake_case",
		"book Bad Name has no toc line",
	)
}

func TestKeeperSweepNeedsBiblio(t *testing.T) {
	if _, err := Render(t.TempDir(), []string{"keeper", "sweep"}); err == nil || !strings.HasPrefix(err.Error(), "how: ") {
		t.Fatalf("err = %v", err)
	}
}
