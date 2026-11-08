// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package hook

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/session"
)

// These fixtures retain the payload shapes captured on agy 1.5.3 in the phase 1
// probe/captures* folders; conversation IDs, user text and paths are sanitized.
func agyCapture(t *testing.T, kind, cwd string, args any) hookEvent {
	t.Helper()
	data, err := os.ReadFile("testdata/agy-captures.json")
	if err != nil {
		t.Fatal(err)
	}
	var captures map[string]map[string]any
	if err := json.Unmarshal(data, &captures); err != nil {
		t.Fatal(err)
	}
	fields, ok := captures[kind]
	if !ok {
		t.Fatalf("missing capture %s", kind)
	}
	fields["workspacePaths"] = []string{cwd, "/ignored/second/workspace"}
	if args != nil {
		fields["toolCall"].(map[string]any)["args"] = args
	}
	raw, _ := json.Marshal(fields)
	p, err := hookAdapterFor("agy").decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	p.Vendor, p.TabID, p.Raw = "agy", "tab-1", raw
	return p
}

func TestAgyCaptureNormalization(t *testing.T) {
	for _, tc := range []struct {
		kind, tool string
		edit       bool
	}{
		{"run_command", "Bash", false}, {"manage_task", "Bash", false},
		{"write_file", "Write", true}, {"edit_file", "Edit", true}, {"read_file", "Read", false},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			p := agyCapture(t, tc.kind, "/workspace/project", nil)
			if p.ToolName != tc.tool || isFileEdit(p) != tc.edit || p.Cwd != "/workspace/project" || p.SessionID != "conversation-a" || p.Prompt == "" || p.TranscriptPath == "" || !p.PreserveSessionState || p.EnforceBibliothek {
				t.Fatalf("normalized payload = %+v", p)
			}
			if tc.tool == "Write" || tc.tool == "Edit" || tc.tool == "Read" {
				if paths := toolFilePaths(p); len(paths) != 1 || paths[0] != "/workspace/project/a.txt" {
					t.Fatalf("paths = %v", paths)
				}
			}
		})
	}
	p, err := (agyHookAdapter{}).decode([]byte(`{"toolCall":{"name":"mcp_example","args":{"path":"x"}}}`))
	if err != nil || p.Cwd != "" || p.ToolName != "mcp_example" || string(p.ToolInput) != `{"path":"x"}` {
		t.Fatalf("passthrough = %+v, %v", p, err)
	}
	p = agyCapture(t, "manage_task", "/workspace/project", map[string]string{"Action": "list"})
	if p.ToolName != "manage_task" {
		t.Fatalf("non-input action became %s", p.ToolName)
	}
}

func emitAgy(t *testing.T, result hookResult) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	agyHookAdapter{}.emit(result)
	w.Close()
	data, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAgyOutputDoesNotBlockAnUndecidedTool(t *testing.T) {
	for _, result := range []hookResult{{}, newHookResult("PreToolUse", "allow", nil), newHookResult("PreToolUse", "allow", []string{"undocumented tool context"}), newHookResult("PostToolUse", "", []string{"undocumented tool context"})} {
		if out := emitAgy(t, result); len(out) != 0 {
			t.Fatalf("no verdict emitted %q; {} blocks agy 1.5.3", out)
		}
	}
	out := emitAgy(t, newHookResult("PreToolUse", "deny", []string{"held"}))
	if strings.TrimSpace(string(out)) != `{"decision":"deny","reason":"held"}` {
		t.Fatalf("deny = %s", out)
	}
	for _, event := range []string{"SessionStart", "UserPromptSubmit"} {
		out := emitAgy(t, newHookResult(event, "", []string{"context"}))
		if strings.TrimSpace(string(out)) != `{"injectSteps":[{"systemMessage":{"systemMessage":"context"}}]}` {
			t.Fatalf("context = %s", out)
		}
	}
	out = emitAgy(t, shellTabFallback("UserPromptSubmit"))
	if !strings.Contains(string(out), `"terminationBehavior":"terminate"`) || !strings.Contains(string(out), `"errorMessage"`) {
		t.Fatalf("prompt block = %s", out)
	}
}

func TestAgyCallsThroughDangerGate(t *testing.T) {
	home, root := launchProjectFixture(t)
	outside := t.TempDir()
	for _, tc := range []struct {
		name, kind string
		args       any
		rule       string
	}{
		{"denied shell", "run_command", map[string]string{"CommandLine": "sudo true"}, "privilege"},
		{"inside deletion", "run_command", map[string]string{"CommandLine": "rm -rf build"}, ""},
		{"outside Cwd deletion", "run_command", map[string]string{"CommandLine": "rm -rf build", "Cwd": outside}, "rm-force"},
		{"outside Cwd find deletion", "run_command", map[string]string{"CommandLine": "find build -delete", "Cwd": outside}, "find-delete"},
		{"inside Cwd find deletion", "run_command", map[string]string{"CommandLine": "find build -delete", "Cwd": root}, ""},
		{"relative protected redirect", "run_command", map[string]string{"CommandLine": "echo changed > hooks.json", "Cwd": filepath.Join(home, ".gemini", "config")}, "guard-config"},
		{"outside absolute deletion", "run_command", map[string]string{"CommandLine": "rm -rf " + shellSingleQuote(filepath.Join(outside, "build"))}, "rm-force"},
		{"protected hooks write", "write_file", map[string]string{"Path": filepath.Join(home, ".gemini", "config", "hooks.json"), "Content": "{}"}, "guard-config"},
		{"protected settings edit", "edit_file", map[string]string{"Path": filepath.Join(home, ".gemini", "antigravity-cli", "settings.json")}, "guard-config"},
		{"workspace hooks write", "write_file", map[string]string{"Path": filepath.Join(root, ".agents", "hooks.json")}, "guard-config"},
		{"secret read", "read_file", map[string]string{"Path": filepath.Join(home, ".ssh", "config")}, "secret-path"},
		{"task input denied", "manage_task", map[string]string{"Action": "send_input", "Input": "sudo true\n"}, "privilege"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := agyCapture(t, tc.kind, root, tc.args)
			got := ""
			if v := dangerDecision(p); v != nil {
				got = v.RuleID
			}
			if got != tc.rule {
				t.Fatalf("rule = %s, want %s; payload %+v", got, tc.rule, p)
			}
		})
	}
}

func TestAgyCloseAndAskGates(t *testing.T) {
	store := openHookTestStore(t)
	root := projectWithBiblio(t)
	run := func(command string) hookResult {
		return hookPreToolUse(store, agyCapture(t, "run_command", root, map[string]string{"CommandLine": command}), root)
	}
	archive := "mv biblio/scratchpad/fix-x biblio/scratchpad/archive/"
	if result := run(archive); result.PermissionDecision != "deny" {
		t.Fatalf("archive before close = %+v", result)
	}
	run("lgrass how task close fix-x")
	if result := run(archive); result.PermissionDecision == "deny" {
		t.Fatalf("archive after close = %+v", result)
	}
	client := startVault(t)
	if err := client.SetPassphrase("pw"); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"run_command", "manage_task"} {
		args := map[string]string{"CommandLine": "git commit -m wip", "Action": "send_input", "Input": "git commit -m wip"}
		p := agyCapture(t, kind, root, args)
		v := verdictFor(t, p, "git commit -m wip")
		if msg := dangerMessageWith(client.SocketPath, p, v); !strings.Contains(msg, "sent the user a request") {
			t.Fatalf("ask = %s", msg)
		}
	}
	asks, err := client.ListAsks("pw")
	if err != nil || len(asks) == 0 || asks[0].Command != "git commit -m wip" || asks[0].Cwd != root {
		t.Fatalf("asks = %+v, %v", asks, err)
	}
}

func TestAgyMemoryGateAndHeldCallsStayClaudeOnly(t *testing.T) {
	store := openHookTestStore(t)
	_, root := memoryGateFixture(t)
	mem, _ := claudeMemoryDir(root)
	for _, p := range []hookEvent{
		agyCapture(t, "run_command", root, map[string]string{"CommandLine": "echo x >> " + filepath.Join(mem, "MEMORY.md")}),
		agyCapture(t, "write_file", root, map[string]string{"Path": filepath.Join(mem, "MEMORY.md"), "Content": "x"}),
	} {
		result := hookPreToolUse(store, p, root)
		if strings.Contains(result.PermissionDecisionReason, "lgrass replay") || strings.Contains(result.PermissionDecisionReason, memoryWriteWord) {
			t.Fatalf("memory gate = %+v", result)
		}
	}
	if calls, err := store.HeldCalls("conversation-a"); err != nil || len(calls) != 0 {
		t.Fatalf("held calls = %v, %v", calls, err)
	}
}

func TestAgyThinkerGateAndRoleOrientation(t *testing.T) {
	store := openHookTestStore(t)
	_, err := store.CreateGroup("review", session.Member{TabID: gateTabLeader, Label: "lead", Vendor: "claude"}, []session.Member{{TabID: gateTabCodex, Label: "tester", Vendor: "agy", Commands: []string{"lgrass how book write"}}})
	if err != nil {
		t.Fatal(err)
	}
	home, root := memoryGateFixture(t)
	dir := filepath.Join(home, ".lemongrass", "orientation")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"tab", "thinker"} {
		if err := os.WriteFile(filepath.Join(dir, "agy-"+role+".md"), []byte(role+" orientation"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p := agyCapture(t, "run_command", root, map[string]string{"CommandLine": "ls"})
	p.TabID = gateTabCodex
	if result := hookPreToolUse(store, p, root); result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "thinker start") {
		t.Fatalf("thinker before commands = %+v", result)
	}
	for _, command := range []string{"lgrass how thinker start", "lgrass how book write"} {
		p = agyCapture(t, "run_command", root, map[string]string{"CommandLine": command})
		p.TabID = gateTabCodex
		hookPreToolUse(store, p, root)
	}
	p = agyCapture(t, "run_command", root, map[string]string{"CommandLine": "ls"})
	p.TabID = gateTabCodex
	if result := hookPreToolUse(store, p, root); result.PermissionDecision == "deny" {
		t.Fatalf("thinker after commands = %+v", result)
	}
	if got := agyOrientation(store, gateTabCodex); got != "thinker orientation" {
		t.Fatalf("thinker orientation = %q", got)
	}
	if got := agyOrientation(store, gateTabOutside); got != "tab orientation" {
		t.Fatalf("tab orientation = %q", got)
	}
}

func TestAgySessionStartAndResumeLaws(t *testing.T) {
	store := openHookTestStore(t)
	home, root := memoryGateFixture(t)
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(home, ".lemongrass", "orientation", "agy-tab.md"), "tab orientation")
	write(filepath.Join(root, "biblio", "laws", "summary.md"), "# Laws summary\n- example law")
	t.Setenv(lawsInPromptEnv, "1")
	p := agyCapture(t, "SessionStart", root, nil)
	result := hookSessionStart(store, p, root)
	if !strings.HasPrefix(result.AdditionalContext, "tab orientation\n\n# Laws summary") {
		t.Fatalf("start = %s", result.AdditionalContext)
	}
	if got := tabSessionOf(t, store, p.TabID); got != "conversation-a" {
		t.Fatalf("session = %s", got)
	}
	t.Setenv(agyResumedEnv, "launch-a")
	p = agyCapture(t, "PreInvocation", root, nil)
	p.Continuation = false
	p.SessionID = "conversation-b"
	result = hookUserPromptSubmit(store, p)
	if !strings.Contains(result.AdditionalContext, "example law") {
		t.Fatalf("resume = %+v", result)
	}
	if again := hookUserPromptSubmit(store, p); strings.Contains(again.AdditionalContext, "example law") {
		t.Fatal("laws repeated within launch")
	}
	t.Setenv(agyResumedEnv, "launch-b")
	if next := hookUserPromptSubmit(store, p); !strings.Contains(next.AdditionalContext, "example law") {
		t.Fatal("next launch did not renew laws")
	}
	recordTabState(store, "UserPromptSubmit", p)
	if got := stateOf(t, store, p.TabID); got != session.StateWorking {
		t.Fatalf("prompt state = %s", got)
	}
	p = agyCapture(t, "Stop", root, nil)
	recordTabState(store, "Stop", p)
	if got := stateOf(t, store, p.TabID); got != session.StateIdle {
		t.Fatalf("stop state = %s", got)
	}
	p.Continuation = true
	p.SessionID = "ignored-continuation"
	recordTabState(store, "UserPromptSubmit", p)
	hookUserPromptSubmit(store, p)
	if got := tabSessionOf(t, store, p.TabID); got != "conversation-b" {
		t.Fatalf("continuation changed session to %s", got)
	}
	if got := stateOf(t, store, p.TabID); got != session.StateIdle {
		t.Fatalf("continuation changed state to %s", got)
	}
	t.Setenv("HOME", t.TempDir())
	if got := agyOrientation(store, p.TabID); got != "" {
		t.Fatalf("missing orientation = %s", got)
	}
	p = agyCapture(t, "SessionStart", root, nil)
	if result := hookSessionStart(store, p, root); !strings.HasPrefix(result.AdditionalContext, "# Laws summary") {
		t.Fatalf("missing orientation dropped laws: %+v", result)
	}
}

func TestAgyContinuationDeliversPendingNotesAfterTools(t *testing.T) {
	store := storeWithPendingMention(t)
	root := t.TempDir()
	p := agyCapture(t, "run_command", root, map[string]string{"CommandLine": "ls"})
	p.TabID = notifyTabTarget
	hookPreToolUse(store, p, root)
	hookPostToolUse(store, p, root)
	p = agyCapture(t, "PreInvocation", root, nil)
	p.TabID, p.Continuation = notifyTabTarget, true
	result := hookUserPromptSubmit(store, p)
	if !strings.Contains(result.AdditionalContext, ": 1 for you") {
		t.Fatalf("notes were lost at tool event: %+v", result)
	}
	if again := hookUserPromptSubmit(store, p); again.hasOutput() {
		t.Fatalf("notes repeated: %+v", again)
	}
}

func TestAgyWorkspaceFallbackAndToolCwdRemainSeparate(t *testing.T) {
	home, root := launchProjectFixture(t)
	t.Setenv(launchProjectEnv, "")
	p := agyCapture(t, "run_command", root, map[string]string{"CommandLine": "rm -rf build"})
	if v := dangerDecision(p); v != nil {
		t.Fatalf("workspace fallback lost exemption: %+v", v)
	}
	p = agyCapture(t, "run_command", root, map[string]string{"CommandLine": "echo changed > hooks.json", "Cwd": filepath.Join(home, ".gemini", "config")})
	if p.ProjectDir != root || p.Cwd == root || strings.HasPrefix(shellCommand(p.ToolName, p.ToolInput), "cd ") {
		t.Fatalf("cwd normalization = %+v", p)
	}
	if v := dangerDecision(p); v == nil || v.RuleID != "guard-config" {
		t.Fatalf("relative protected write = %+v", v)
	}
}
