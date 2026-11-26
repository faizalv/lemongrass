// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/faizalv/lemongrass/internal/guard"
	"github.com/faizalv/lemongrass/internal/project"
	"github.com/faizalv/lemongrass/internal/session"
)

// secondProject registers another project beside the launch project of launchProjectFixture.
func secondProject(t *testing.T, withBiblio bool) project.Project {
	t.Helper()
	dir := t.TempDir()
	if withBiblio {
		if err := os.MkdirAll(filepath.Join(dir, "biblio", "scratchpad", "fix-x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Register(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStoreProjectFollowsTheTab(t *testing.T) {
	_, root := launchProjectFixture(t)
	other := secondProject(t, false)

	got, err := storeProject(hookEvent{TabID: "tab-1", Cwd: other.Path}, other.Path)
	if err != nil || got.Path != resolvedPath(t, root) {
		t.Errorf("tab call from another project stored in %q (%v), want the launch project %q", got.Path, err, root)
	}
	got, err = storeProject(hookEvent{Cwd: other.Path}, other.Path)
	if err != nil || got.ID != other.ID {
		t.Errorf("tab-less call stored in %q (%v), want its cwd project %q", got.Path, err, other.Path)
	}
}

func resolvedPath(t *testing.T, p string) string {
	t.Helper()
	proj, err := project.Resolve(p)
	if err != nil {
		t.Fatal(err)
	}
	return proj.Path
}

const scopeRunChildEnv = "LGRASS_TEST_SCOPE_CHILD"

// runHookChild runs Run in a child process for a Codex PreToolUse payload, since Run exits the process.
func runHookChild(t *testing.T, home, launchRoot, tab string, payload map[string]any) string {
	t.Helper()
	input, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRunCodexCallFromAnotherProject$")
	cmd.Env = append(os.Environ(), scopeRunChildEnv+"="+tab, "HOME="+home, launchProjectEnv+"="+launchRoot, claudeProjectDirEnv+"=", "LGRASS_HOOK_VENDOR=")
	cmd.Stdin = bytes.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("child run failed: %v", err)
	}
	return string(out)
}

func TestRunCodexCallFromAnotherProject(t *testing.T) {
	if tab := os.Getenv(scopeRunChildEnv); tab != "" {
		Run([]string{"PreToolUse", "--vendor", "codex"}, tab)
		return
	}
	home, root := launchProjectFixture(t)
	launch, err := project.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	other := secondProject(t, false)
	shell := func(sessionID string) map[string]any {
		return map[string]any{
			"session_id": sessionID, "cwd": other.Path, "hook_event_name": "PreToolUse", "turn_id": "turn-1",
			"tool_use_id": "call-1", "tool_name": "Bash", "tool_input": map[string]string{"command": "ls"},
		}
	}

	runHookChild(t, home, root, gateTabOutside, shell("s-outside"))
	launchStore, err := session.Open(session.DBPath(), launch.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer launchStore.Close()
	otherStore, err := session.Open(session.DBPath(), other.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer otherStore.Close()
	// The tab session is what restores a Codex tab at startup, read from the launch project's store.
	if got := tabSessionOf(t, launchStore, gateTabOutside); got != "s-outside" {
		t.Errorf("launch project tab session = %q, want s-outside", got)
	}
	if got := tabSessionOf(t, otherStore, gateTabOutside); got != "" {
		t.Errorf("the other project recorded tab session %q for a tab launched elsewhere", got)
	}

	// The thinker gate reads membership from the launch project's store, so a thinker working in another project is still held to it.
	if _, err := launchStore.CreateGroup("review", session.Member{TabID: gateTabLeader, Label: "lead", Vendor: "claude"}, []session.Member{
		{TabID: gateTabCodex, Label: "tester", Vendor: "codex"},
	}); err != nil {
		t.Fatal(err)
	}
	if out := runHookChild(t, home, root, gateTabCodex, shell("s-thinker")); !strings.Contains(out, `"permissionDecision":"deny"`) {
		t.Errorf("a thinker's call from another project was not gated: %s", out)
	}
}

func TestCloseGateCoversATouchedProject(t *testing.T) {
	_, root := launchProjectFixture(t)
	other := secondProject(t, true)
	store := openHookTestStore(t)
	// openHookTestStore moved HOME; the registry must live under the same home.
	if _, err := project.Register(root); err != nil {
		t.Fatal(err)
	}
	if _, err := project.Register(other.Path); err != nil {
		t.Fatal(err)
	}
	command := "cd " + other.Path + " && mv biblio/scratchpad/fix-x biblio/scratchpad/archive/"
	result := hookPreToolUse(store, preTool(gateTabOutside, "Bash", map[string]string{"command": command}), root)
	if result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "fix-x") {
		t.Errorf("archiving a task in a touched project without its close was not denied: %+v", result)
	}
}

func TestTempWriteDeniedInATouchedProject(t *testing.T) {
	_, root := launchProjectFixture(t)
	other := secondProject(t, true)
	target := filepath.Join(os.TempDir(), "lgrass-scope-probe.txt")
	payload := hookEvent{TabID: gateTabOutside, ToolName: "Write", Cwd: other.Path, ToolInput: json.RawMessage(`{"file_path":"` + target + `","content":"x"}`)}
	roots := callRoots(payload, root)
	if len(roots) != 2 {
		t.Fatalf("roots = %v, want the launch project and the cwd's project", roots)
	}
	if deny := tempWriteDeny(payload, roots...); deny == "" {
		t.Errorf("a temp write from a touched biblio project was not denied")
	}
	inside := hookEvent{ToolName: "Write", Cwd: other.Path, ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(other.Path, "a.txt") + `","content":"x"}`)}
	if deny := tempWriteDeny(inside, roots...); deny != "" {
		t.Errorf("a write inside a touched project under the temp dir was denied: %s", deny)
	}
}

func TestDangerDecisionTakesTheStrictestPolicy(t *testing.T) {
	_, root := launchProjectFixture(t)
	launch, err := project.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	other := secondProject(t, false)
	policies := map[string]guard.Policy{other.ID: {Binaries: []string{"make"}}}
	saved := fetchPolicy
	fetchPolicy = func(id string) guard.Policy { return policies[id] }
	t.Cleanup(func() { fetchPolicy = saved })

	if v := dangerDecision(bashPayloadAt(root, "make build")); v != nil {
		t.Fatalf("make in the launch project = %+v, want allowed", v)
	}
	v := dangerDecision(bashPayloadAt(root, "cd "+other.Path+" && make build"))
	if v == nil || v.RuleID != "custom-binary" {
		t.Errorf("make reaching a project that blocks it = %+v, want custom-binary", v)
	}
	policies[launch.ID] = guard.Policy{Rules: map[string]string{"rm-plain": "deny"}}
	v = dangerDecision(bashPayloadAt(other.Path, "rm notes.txt"))
	if v == nil || v.Mode != guard.ModeDeny {
		t.Errorf("rm under the launch project's deny = %+v, want deny", v)
	}
}

func bashPayloadAt(cwd, command string) hookEvent {
	raw, _ := json.Marshal(map[string]string{"command": command})
	return hookEvent{SessionID: "s", TabID: "tab-1", ToolName: "Bash", ToolInput: raw, Cwd: cwd, Vendor: "claude"}
}

func TestShellWritesInATouchedProjectReachTheTabsLog(t *testing.T) {
	_, root := launchProjectFixture(t)
	store := openHookTestStore(t)
	if _, err := project.Register(root); err != nil {
		t.Fatal(err)
	}
	otherDir := t.TempDir()
	gitRepo(t, otherDir)
	if _, err := project.Register(otherDir); err != nil {
		t.Fatal(err)
	}
	pre := preTool("tab-1", "Bash", map[string]string{"command": "cd " + otherDir + " && ./gen.sh"})
	pre.ToolUseID = "call-1"
	pre.Cwd = root
	hookPreToolUse(store, pre, root)
	made := filepath.Join(otherDir, "generated.go")
	writeFileAt(t, made, time.Now())
	hookPostToolUse(store, pre, root)

	touches, _ := store.FileActivitySince(time.Now().Add(-time.Minute))
	var logged []string
	for _, touch := range touches {
		logged = append(logged, touch.FilePath)
	}
	if !slices.ContainsFunc(logged, func(p string) bool {
		return filepath.Base(p) == "generated.go" && strings.HasPrefix(p, resolvedPath(t, otherDir))
	}) {
		t.Errorf("logged %v, want %s", logged, made)
	}
}
