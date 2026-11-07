// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// These cases spell paths the Unix way, where Codex 0.167 was probed.

//go:build !windows

package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/guard"
	"github.com/faizalv/lemongrass/internal/project"
)

func patchInput(lines ...string) map[string]string {
	return map[string]string{"command": "*** Begin Patch\n" + strings.Join(lines, "\n") + "\n*** End Patch"}
}

// launchProjectFixture registers a launch project under a fresh home and points LGRASS_PROJECT_DIR at it, with CLAUDE_PROJECT_DIR unset as in a Codex tab.
func launchProjectFixture(t *testing.T) (home, root string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := project.Register(root); err != nil {
		t.Fatal(err)
	}
	t.Setenv(launchProjectEnv, root)
	t.Setenv(claudeProjectDirEnv, "")
	return home, root
}

func TestCodexCallsThroughDangerGate(t *testing.T) {
	home, root := launchProjectFixture(t)
	elsewhere := t.TempDir()
	cases := []struct {
		name  string
		cwd   string
		tool  string
		input map[string]string
		want  string
	}{
		{"shell read of a secret", root, "Bash", map[string]string{"command": "cat ~/.ssh/config"}, "secret-path"},
		{"shell read", root, "Bash", map[string]string{"command": "head -n 5 README.md"}, ""},
		{"shell rm inside the launch project", root, "Bash", map[string]string{"command": "rm -rf build"}, ""},
		{"shell rm outside the launch project", elsewhere, "Bash", map[string]string{"command": "rm -rf build"}, "rm-force"},
		{"patch adds a project file", root, "apply_patch", patchInput("*** Add File: "+filepath.Join(root, "probe-edit.txt"), "+x"), ""},
		{"patch adds a secret file", root, "apply_patch", patchInput("*** Add File: "+filepath.Join(home, ".ssh", "authorized_keys"), "+x"), "secret-path"},
		{"patch edits the codex hooks", root, "apply_patch", patchInput("*** Update File: "+filepath.Join(home, ".codex", "hooks.json"), "@@", "-a", "+b"), "guard-config"},
		{"patch moves a file into a secret dir", root, "apply_patch", patchInput("*** Update File: "+filepath.Join(root, "a.txt"), "*** Move to: "+filepath.Join(home, ".ssh", "config"), "@@", "-a", "+b"), "secret-path"},
		{"patch deletes a project file", root, "apply_patch", patchInput("*** Delete File: " + filepath.Join(root, "old.txt")), ""},
		{"patch deletes a file outside the launch project", root, "apply_patch", patchInput("*** Delete File: " + filepath.Join(elsewhere, "old.txt")), "rm-plain"},
		{"patch deletes a secret file", root, "apply_patch", patchInput("*** Delete File: " + filepath.Join(home, ".ssh", "id_rsa")), "secret-path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := codexPayload(t, "PreToolUse", "session-a", tc.cwd, tc.tool, tc.input)
			got := ""
			if verdict := dangerDecision(payload); verdict != nil {
				got = verdict.RuleID
			}
			if got != tc.want {
				t.Errorf("rule = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCodexPatchDeletionWithoutLaunchProject(t *testing.T) {
	_, root := launchProjectFixture(t)
	t.Setenv(launchProjectEnv, "")
	payload := codexPayload(t, "PreToolUse", "session-a", root, "apply_patch", patchInput("*** Delete File: "+filepath.Join(root, "old.txt")))
	if verdict := dangerDecision(payload); verdict == nil || verdict.RuleID != "rm-plain" {
		t.Errorf("verdict = %+v, want rm-plain with no launch project", verdict)
	}
}

func TestLaunchProjectEnvBeforeClaudeFallback(t *testing.T) {
	_, root := launchProjectFixture(t)
	other := t.TempDir()
	if _, err := project.Register(other); err != nil {
		t.Fatal(err)
	}

	t.Setenv(claudeProjectDirEnv, other)
	if got := launchProject().Path; got != root {
		t.Errorf("with both set, launch project = %q, want %q from %s", got, root, launchProjectEnv)
	}
	t.Setenv(launchProjectEnv, "")
	if got := launchProject().Path; got != other {
		t.Errorf("with only %s set, launch project = %q, want %q", claudeProjectDirEnv, got, other)
	}
}

func TestCodexAskModeShellFilesAnAsk(t *testing.T) {
	client := startVault(t)
	if err := client.SetPassphrase("pw"); err != nil {
		t.Fatal(err)
	}
	payload := codexPayload(t, "PreToolUse", "session-a", t.TempDir(), "Bash", map[string]string{"command": "git commit -m wip"})
	verdict := verdictFor(t, payload, "git commit -m wip")

	msg := dangerMessageWith(client.SocketPath, payload, verdict)
	if !strings.Contains(msg, "sent the user a request") {
		t.Fatalf("message = %q, want an ask filed", msg)
	}
	asks, err := client.ListAsks("pw")
	if err != nil || len(asks) != 1 || asks[0].Command != "git commit -m wip" || asks[0].TabID != "tab-1" {
		t.Fatalf("ListAsks = %+v, %v", asks, err)
	}
}

// Asks are filed only for shell commands, so a patch refused in ask mode keeps the guard's message, as Claude's Write does.
func TestCodexPatchRefusalKeepsTheGuardMessage(t *testing.T) {
	client := startVault(t)
	_, _ = launchProjectFixture(t)
	elsewhere := t.TempDir()
	payload := codexPayload(t, "PreToolUse", "session-a", elsewhere, "apply_patch", patchInput("*** Delete File: "+filepath.Join(elsewhere, "old.txt")))
	verdict := dangerDecision(payload)
	if verdict == nil || verdict.Mode != guard.ModeAsk {
		t.Fatalf("verdict = %+v, want an ask-mode deletion", verdict)
	}
	if msg := dangerMessageWith(client.SocketPath, payload, *verdict); msg != verdict.Message() {
		t.Errorf("message = %q, want the guard's own", msg)
	}
}

func TestCodexCloseGateDeniesArchiveUntilHowRuns(t *testing.T) {
	store := openHookTestStore(t)
	projectPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectPath, "biblio"), 0o755); err != nil {
		t.Fatal(err)
	}
	shell := func(command string) hookResult {
		return hookPreToolUse(store, codexPayload(t, "PreToolUse", "session-a", projectPath, "Bash", map[string]string{"command": command}), projectPath)
	}
	archive := "mv biblio/scratchpad/fix-x biblio/scratchpad/archive/"

	if result := shell(archive); result.PermissionDecision != "deny" {
		t.Fatalf("archive before how = %q, want deny", result.PermissionDecision)
	}
	shell("lgrass how task close fix-x")
	if result := shell(archive); result.PermissionDecision == "deny" {
		t.Fatalf("archive after how denied: %s", result.PermissionDecisionReason)
	}
}

// The memory gate, and the held calls it makes, is Claude's: a Codex call is neither gated by it nor held.
func TestCodexCallIntoClaudeMemoryIsNotHeld(t *testing.T) {
	store := openHookTestStore(t)
	_, projectPath := memoryGateFixture(t)
	memDir, _ := claudeMemoryDir(projectPath)
	payload := codexPayload(t, "PreToolUse", "session-a", projectPath, "Bash", map[string]string{"command": "echo x >> " + filepath.Join(memDir, "MEMORY.md")})

	result := hookPreToolUse(store, payload, projectPath)
	if strings.Contains(result.PermissionDecisionReason, "lgrass replay") || strings.Contains(result.PermissionDecisionReason, memoryWriteWord) {
		t.Errorf("Codex call met the memory gate: %s", result.PermissionDecisionReason)
	}
	if calls, err := store.HeldCalls("session-a"); err != nil || len(calls) != 0 {
		t.Errorf("HeldCalls = %d, %v, want none", len(calls), err)
	}
}
