// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/faizalv/lemongrass/internal/bibliothek"
)

var installedOrientations = []string{"tab.md", "thinker.md", "codex-tab.md", "codex-thinker.md", "agy-tab.md", "agy-thinker.md"}

// embeddedOrientation is the expected installed text for an installed name, codex- prefixed for Codex and agy- for Antigravity.
func embeddedOrientation(t *testing.T, name string) string {
	t.Helper()
	vendor, source := "claude", name
	for _, v := range []string{"codex", "agy"} {
		if rest, ok := strings.CutPrefix(name, v+"-"); ok {
			vendor, source = v, rest
		}
	}
	data, err := orientationFS.ReadFile("orientation/" + source)
	if err != nil {
		t.Fatal(err)
	}
	convention, err := bibliothek.Prompt()
	if err != nil {
		t.Fatal(err)
	}
	return string(forVendor(data, vendor)) + bibliothekHeading + string(convention)
}

func TestOrientationCarriesBibliothekWithoutTheTrigger(t *testing.T) {
	k, _ := fakeHome(t)
	if err := k.reconcile(); err != nil {
		t.Fatal(err)
	}
	for _, name := range installedOrientations {
		data, err := os.ReadFile(filepath.Join(k.orientationDir(), name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "# The bibliothek convention") || !strings.Contains(text, "## Tier 2") {
			t.Errorf("%s does not carry bibliothek", name)
		}
		if strings.Contains(text, "MANDATORY: invoke bibliothek") || strings.Contains(text, "name: bibliothek") {
			t.Errorf("%s carries the autoload trigger or the skill frontmatter", name)
		}
	}
}

func TestOrientationIsInstalledForTabsAndThinkers(t *testing.T) {
	k, _ := fakeHome(t)
	if err := k.reconcile(); err != nil {
		t.Fatal(err)
	}
	for _, name := range installedOrientations {
		got, err := os.ReadFile(filepath.Join(k.orientationDir(), name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != embeddedOrientation(t, name) {
			t.Errorf("%s differs from its embedded copy", name)
		}
	}
}

func TestOrientationIsAdaptedPerVendor(t *testing.T) {
	k, _ := fakeHome(t)
	if err := k.reconcile(); err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(k.orientationDir(), name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for _, name := range installedOrientations {
		text := read(name)
		orientation, _, _ := strings.Cut(text, bibliothekHeading)
		if strings.Contains(orientation, "<!--") || strings.Contains(text, "\n\n\n") {
			t.Errorf("%s keeps a vendor marker or a doubled gap", name)
		}
	}
	for _, role := range []string{"tab.md", "thinker.md"} {
		// Only the orientation itself; the appended bibliothek text names agents on purpose.
		claudeText, _, _ := strings.Cut(read(role), bibliothekHeading)
		codexText, _, _ := strings.Cut(read("codex-"+role), bibliothekHeading)
		agyText, _, _ := strings.Cut(read("agy-"+role), bibliothekHeading)
		for _, harness := range []string{"ToolSearch", "Claude Code's", "memory gate"} {
			if !strings.Contains(claudeText, harness) {
				t.Errorf("%s lost %q", role, harness)
			}
			if strings.Contains(codexText, harness) {
				t.Errorf("codex-%s carries %q", role, harness)
			}
			if strings.Contains(agyText, harness) {
				t.Errorf("agy-%s carries %q", role, harness)
			}
		}
		if !strings.Contains(codexText, "changes to Codex's config, hooks or the lemongrass binaries") {
			t.Errorf("codex-%s does not name Codex's config in the blocked list", role)
		}
		for _, own := range []string{"changes to Antigravity's settings, hooks or the lemongrass binaries", "A GEMINI.md or AGENTS.md file", "running Claude, Codex or Antigravity"} {
			if !strings.Contains(agyText, own) {
				t.Errorf("agy-%s lacks %q", role, own)
			}
		}
		// A mid-turn note reaches Antigravity at its next model call, so its text never promises one at the next tool call.
		for _, other := range []string{"Codex's config", "CLAUDE.md", "at your next tool call"} {
			if strings.Contains(agyText, other) {
				t.Errorf("agy-%s carries %q", role, other)
			}
		}
	}
}

func TestForVendorKeepsOneParagraphGap(t *testing.T) {
	src := "a\n\n<!-- claude -->\nb\n<!-- end -->\n<!-- codex -->\nc\n<!-- end -->\n\n<!-- claude -->\nd\n<!-- end -->\n\ne\n"
	if got := string(forVendor([]byte(src), "claude")); got != "a\n\nb\n\nd\n\ne\n" {
		t.Errorf("claude = %q", got)
	}
	if got := string(forVendor([]byte(src), "codex")); got != "a\n\nc\n\ne\n" {
		t.Errorf("codex = %q", got)
	}
}

func TestForVendorBlockNamesSeveralVendors(t *testing.T) {
	src := "a\n\n<!-- claude -->\nb\n<!-- end -->\n<!-- codex agy -->\nc\n<!-- end -->\n<!-- agy -->\nd\n<!-- end -->\n\ne\n"
	for vendor, want := range map[string]string{"claude": "a\n\nb\n\ne\n", "codex": "a\n\nc\n\ne\n", "agy": "a\n\nc\nd\n\ne\n"} {
		if got := string(forVendor([]byte(src), vendor)); got != want {
			t.Errorf("%s = %q, want %q", vendor, got, want)
		}
	}
}

func TestMatchingOrientationIsNotRewritten(t *testing.T) {
	k, _ := fakeHome(t)
	if err := k.reconcile(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(k.orientationDir(), "tab.md")
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if err := k.reconcile(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Error("an unchanged orientation was rewritten")
	}
}

func TestWatcherRestoresEditedOrientation(t *testing.T) {
	k, _ := fakeHome(t)
	w := startWatcher(t, k)

	path := filepath.Join(k.orientationDir(), "thinker.md")
	want := embeddedOrientation(t, "thinker.md")
	restored := func() bool {
		data, err := os.ReadFile(path)
		return err == nil && string(data) == want
	}
	w.waitFor(t, "initial install", restored)
	if err := os.WriteFile(path, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.waitFor(t, "orientation restored after an edit", restored)
	if err := os.RemoveAll(k.orientationDir()); err != nil {
		t.Fatal(err)
	}
	w.waitFor(t, "orientation restored after deletion", restored)
}
