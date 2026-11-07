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

var installedOrientations = []string{"tab.md", "thinker.md", "codex-tab.md", "codex-thinker.md"}

// embeddedOrientation is the expected installed text for an installed name, codex- prefixed for Codex.
func embeddedOrientation(t *testing.T, name string) string {
	t.Helper()
	vendor, source := "claude", name
	if rest, ok := strings.CutPrefix(name, "codex-"); ok {
		vendor, source = "codex", rest
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
		if strings.Contains(text, "<!-- claude -->") || strings.Contains(text, "<!-- codex -->") || strings.Contains(text, vendorBlockEnd) || strings.Contains(text, "\n\n\n") {
			t.Errorf("%s keeps a vendor marker or a doubled gap", name)
		}
	}
	for _, role := range []string{"tab.md", "thinker.md"} {
		// Only the orientation itself; the appended bibliothek text names agents on purpose.
		claudeText, _, _ := strings.Cut(read(role), bibliothekHeading)
		codexText, _, _ := strings.Cut(read("codex-"+role), bibliothekHeading)
		for _, harness := range []string{"ToolSearch", "Claude Code's"} {
			if !strings.Contains(claudeText, harness) {
				t.Errorf("%s lost %q", role, harness)
			}
			if strings.Contains(codexText, harness) {
				t.Errorf("codex-%s carries %q", role, harness)
			}
		}
		if !strings.Contains(codexText, "changes to Codex's config, hooks or the lemongrass binaries") {
			t.Errorf("codex-%s does not name Codex's config in the blocked list", role)
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
