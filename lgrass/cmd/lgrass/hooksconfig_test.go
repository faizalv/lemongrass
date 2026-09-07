package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func settingsPathForTest(t *testing.T) string {
	t.Helper()
	home := os.Getenv("HOME")
	return filepath.Join(home, ".claude", "settings.json")
}

func readHooks(t *testing.T, path string) map[string][]hookGroup {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	var hooks map[string][]hookGroup
	if err := json.Unmarshal(raw["hooks"], &hooks); err != nil {
		t.Fatalf("parsing hooks in %s: %v", path, err)
	}
	return hooks
}

func TestEnsureClaudeHooksCreatesAllFiveOnAFreshHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := ensureClaudeHooks(); err != nil {
		t.Fatalf("ensureClaudeHooks: %v", err)
	}

	hooks := readHooks(t, settingsPathForTest(t))
	for _, spec := range lgrassHookSpecs {
		if !hasLgrassHook(hooks[spec.event], spec.event) {
			t.Errorf("no lgrass hook registered for %s", spec.event)
		}
	}
	if got := hooks["PreToolUse"][0].Matcher; got != "Write|Edit" {
		t.Errorf("PreToolUse matcher = %q, want %q", got, "Write|Edit")
	}
}

func TestEnsureClaudeHooksIsIdempotent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := ensureClaudeHooks(); err != nil {
		t.Fatalf("ensureClaudeHooks (first): %v", err)
	}
	if err := ensureClaudeHooks(); err != nil {
		t.Fatalf("ensureClaudeHooks (second): %v", err)
	}

	hooks := readHooks(t, settingsPathForTest(t))
	for _, spec := range lgrassHookSpecs {
		if got := len(hooks[spec.event]); got != 1 {
			t.Errorf("%s has %d hook groups after registering twice, want 1", spec.event, got)
		}
	}
}

func TestEnsureClaudeHooksPreservesExistingSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := settingsPathForTest(t)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	existing := `{
		"editorMode": "vim",
		"hooks": {
			"PreToolUse": [
				{"matcher": "Bash", "hooks": [{"type": "command", "command": "some-other-tool hook"}]}
			],
			"SessionStart": [
				{"hooks": [{"type": "command", "command": "/opt/lgrass/lgrass hook SessionStart"}]}
			]
		}
	}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := ensureClaudeHooks(); err != nil {
		t.Fatalf("ensureClaudeHooks: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading %s: %v", settingsPath, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing %s: %v", settingsPath, err)
	}
	var editorMode string
	if err := json.Unmarshal(raw["editorMode"], &editorMode); err != nil || editorMode != "vim" {
		t.Errorf("editorMode = %q, err %v, want %q preserved", editorMode, err, "vim")
	}

	hooks := readHooks(t, settingsPath)
	if len(hooks["PreToolUse"]) != 2 {
		t.Fatalf("PreToolUse has %d groups, want 2 (existing Bash matcher + new lgrass entry)", len(hooks["PreToolUse"]))
	}
	if hooks["PreToolUse"][0].Matcher != "Bash" {
		t.Errorf("existing PreToolUse Bash group was not preserved in place")
	}
	if len(hooks["SessionStart"]) != 1 {
		t.Errorf("SessionStart has %d groups, want 1 (already-registered entry left alone, not duplicated)", len(hooks["SessionStart"]))
	}
	for _, spec := range lgrassHookSpecs {
		if !hasLgrassHook(hooks[spec.event], spec.event) {
			t.Errorf("no lgrass hook registered for %s", spec.event)
		}
	}
}
