// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package how

import (
	"strings"
	"testing"
)

func TestManualsRender(t *testing.T) {
	root := biblioRoot(t)
	cases := map[string][]string{
		"ask request":      {"lgrass ask <path>", "operations: [select]", "methods: [GET, POST]", "Removed tables", "type: db", "type: rester", "type: command", "lgrass ask show <id>", "Do not ask again with a variation"},
		"berries map":      {".berries/map.yaml", "status: proposed", "Only the human sets `approved`", "lgrass berries map <short-id>"},
		"berries run":      {"lgrass berries run <short-id>", "lgrass berries seeds", "control_failed", "lgrass how berries stain"},
		"berries stain":    {"lgrass berries stain <stain-id>", "open_proof_gap", "never edit it so the stain goes away", "Berries pane"},
		"berries fix":      {"lgrass berries replay <short-id> <stain-id>", "lgrass berries run <short-id>"},
		"git resolve":      {"lgrass git conflict show", "lgrass git conflict decide", "lgrass git conflict ask", "lgrass git conflict submit", "never stage"},
		"leader create":    {"lgrass workgroup create <config>", "lgrass how thinker start", "lgrass how task close <slug>", "lgrass how task handover <slug>"},
		"thinker start":    {"lgrass workgroup thread", "wait for the leader's go"},
		"write activities": {"# Writing activities.md", "- YYYY-MM-DD: <what happened>", "never a transcript"},
		"write chapter":    {"Split.", "never how it broke", "toc.md"},
		"write debug":      {"# Writing debug.md", "root cause"},
		"write handover":   {"# Writing a handover", "biblio/handover/<slug>.md", "## Status (authoritative", "N. <slug> [tags] -> <slug>.md"},
		"write law":        {"explicit yes", "biblio/laws/summary.md", "Never write a law to memory"},
		"write prd":        {"# Writing prd.md", "biblio/scratchpad/<slug>/prd.md", "## Problem", "### Open decisions", "- [ ] User approval"},
		"write recon":      {"# Writing recon.md", "still open"},
	}
	for cmd, wants := range cases {
		out := render(t, root, strings.Fields(cmd)...)
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Errorf("%s missing %q", cmd, want)
			}
		}
		if _, err := Render(root, append(strings.Fields(cmd), "extra")); err == nil {
			t.Errorf("%s accepted an argument", cmd)
		}
	}
}

func TestRetiredDomainsPointAtWrite(t *testing.T) {
	root := biblioRoot(t)
	for cmd, want := range map[string]string{"book write": "lgrass how write chapter", "laws write": "lgrass how write law"} {
		if _, err := Render(root, strings.Fields(cmd)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want a pointer at %q", cmd, err, want)
		}
	}
}

// The procedures print the same fragment as `lgrass how write`, so a format has one home.
func TestProceduresShareWriteFormats(t *testing.T) {
	root := t.TempDir()
	task(t, root, "feat", "- [x] a", "- [ ] b")
	prd := fragment("write/prd.md", nil)
	if out := render(t, root, "work", "task", "fresh"); !strings.Contains(out, prd) {
		t.Errorf("work task lacks the write prd fragment")
	}
	if out := render(t, root, "write", "prd"); !strings.Contains(out, prd) {
		t.Errorf("write prd lacks its fragment")
	}
	handover := strings.ReplaceAll(fragment("write/handover.md", nil), "<slug>", "feat")
	if out := render(t, root, "task", "handover", "feat"); !strings.Contains(out, handover) || strings.Contains(out, "<slug>") {
		t.Errorf("task handover lacks the write handover fragment with the slug filled in")
	}
}

func TestHandoverNeedsPRD(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "biblio/scratchpad/idea/notes.md", "loose")
	if out := render(t, root, "task", "handover", "idea"); !strings.Contains(out, "write the PRD first") {
		t.Fatalf("out = %q", out)
	}
}

func TestHandoverRefusesFinishedWork(t *testing.T) {
	root := t.TempDir()
	task(t, root, "done", "- [x] a", "- [x] b")
	if out := render(t, root, "task", "handover", "done"); !strings.Contains(out, "never written for finished work") {
		t.Fatalf("out = %q", out)
	}
}

func TestHandoverNew(t *testing.T) {
	root := t.TempDir()
	task(t, root, "feat", "- [x] a", "- [ ] b")
	out := render(t, root, "task", "handover", "feat")
	for _, want := range []string{"New handover for feat: 1 of 2", "biblio/handover/feat.md", "-> feat.md", "i-write-memory", "**Books.**", "lgrass how write chapter"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}

func TestHandoverUpdateChecksWhiteboard(t *testing.T) {
	root := t.TempDir()
	task(t, root, "feat", "- [ ] b")
	writeFile(t, root, "biblio/handover/feat.md", "# feat")
	out := render(t, root, "task", "handover", "feat")
	if !strings.Contains(out, "Rewrite the Status") || !strings.Contains(out, "has no row") || !strings.Contains(out, "lgrass how write chapter") {
		t.Fatalf("out = %q", out)
	}
	writeFile(t, root, "biblio/handover/whiteboard.md", "1. feat [x] -> feat.md\n")
	if out := render(t, root, "task", "handover", "feat"); !strings.Contains(out, "stays") {
		t.Fatalf("with row out = %q", out)
	}
}
