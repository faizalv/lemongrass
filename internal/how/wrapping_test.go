// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package how

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixClock(t *testing.T, at time.Time) {
	t.Helper()
	old := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = old })
}

func TestWrappingCreatesATimestampedFile(t *testing.T) {
	root := biblioRoot(t)
	fixClock(t, time.Date(2026, 11, 24, 21, 30, 0, 0, time.Local))
	out := render(t, root, "work", "wrapping", "feat")
	rel := "biblio/scratchpad/feat/wraps/2026-11-24-2130.md"
	wantAll(t, out, "Wrap file created: `"+rel+"`", "List what is not yet persisted", "Fill the wrap file", "lgrass how task handover <slug>")
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "# Wrap of feat, 2026-11-24 21:30\n") {
		t.Errorf("skeleton = %q", data)
	}
}

func TestWrappingNeverOverwritesAnEarlierWrap(t *testing.T) {
	root := biblioRoot(t)
	fixClock(t, time.Date(2026, 11, 24, 21, 30, 0, 0, time.Local))
	render(t, root, "work", "wrapping", "feat")
	first := filepath.Join(root, "biblio/scratchpad/feat/wraps/2026-11-24-2130.md")
	if err := os.WriteFile(first, []byte("filled"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := render(t, root, "work", "wrapping", "feat")
	wantAll(t, out, "wraps/2026-11-24-2130-2.md")
	if data, _ := os.ReadFile(first); string(data) != "filled" {
		t.Errorf("first wrap overwritten: %q", data)
	}
}

func TestWrappingNeedsASlugAndALiveTask(t *testing.T) {
	root := biblioRoot(t)
	if _, err := Render(root, []string{"work", "wrapping"}); err == nil || !strings.Contains(err.Error(), "lgrass how work wrapping <slug>") {
		t.Errorf("no slug: err %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "biblio/scratchpad/archive/old"), 0o755); err != nil {
		t.Fatal(err)
	}
	wantAll(t, render(t, root, "work", "wrapping", "old"), "archived task")
	if _, err := os.Stat(filepath.Join(root, "biblio/scratchpad/old")); err == nil {
		t.Error("wrapping an archived task created a live directory")
	}
}

func TestEveryDomainHasASentence(t *testing.T) {
	for _, d := range Domains() {
		if d.Summary == "" || !strings.HasSuffix(d.Summary, ".") {
			t.Errorf("domain %s summary = %q", d.Name, d.Summary)
		}
	}
	if len(domainSummaries) != len(domains) {
		t.Errorf("%d sentences for %d domains", len(domainSummaries), len(domains))
	}
}

func TestListingsAlignTheirColumns(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{nil, {"task"}, {"work"}} {
		lines := strings.Split(strings.TrimSpace(render(t, root, args...)), "\n")[1:]
		col := -1
		for _, line := range lines {
			fields := strings.Fields(line)
			at := strings.Index(line, fields[1])
			if col >= 0 && at != col {
				t.Errorf("%v: %q starts its text at %d, want %d", args, line, at, col)
			}
			col = at
		}
	}
}
