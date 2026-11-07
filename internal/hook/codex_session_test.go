// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"encoding/json"
	"testing"

	"github.com/faizalv/lemongrass/internal/session"
)

// codexPayload builds a hook payload in the shape Codex 0.167 sends, decoded the way Run decodes it for a Codex tab.
func codexPayload(t *testing.T, event, sessionID, cwd, tool string, input any) hookEvent {
	t.Helper()
	fields := map[string]any{
		"session_id":      sessionID,
		"transcript_path": "/home/user/.codex/sessions/2026/11/07/rollout-" + sessionID + ".jsonl",
		"cwd":             cwd,
		"hook_event_name": event,
		"model":           "gpt-6.1-sol",
		"permission_mode": "bypassPermissions",
	}
	if tool != "" {
		in, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		fields["turn_id"] = "turn-1"
		fields["tool_use_id"] = "call-1"
		fields["tool_name"] = tool
		fields["tool_input"] = json.RawMessage(in)
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := codexHookAdapter{}.decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	payload.TabID = "tab-1"
	payload.Vendor = "codex"
	payload.Raw = raw
	return payload
}

func tabSessionOf(t *testing.T, store *session.Store, tab string) string {
	t.Helper()
	id, err := store.SessionForTab(tab)
	if err != nil {
		t.Fatalf("SessionForTab: %v", err)
	}
	return id
}

type codexTab struct {
	t       *testing.T
	store   *session.Store
	project string
}

func (c codexTab) start(sessionID, source string) {
	p := codexPayload(c.t, "SessionStart", sessionID, c.project, "", nil)
	p.SessionSource = source
	hookSessionStart(c.store, p, c.project)
}

func (c codexTab) prompt(sessionID string) {
	hookUserPromptSubmit(c.store, codexPayload(c.t, "UserPromptSubmit", sessionID, c.project, "", nil))
}

func (c codexTab) shell(sessionID, command string) {
	hookPreToolUse(c.store, codexPayload(c.t, "PreToolUse", sessionID, c.project, "Bash", map[string]string{"command": command}), c.project)
}

func (c codexTab) end(sessionID string) {
	hookSessionEnd(c.store, codexPayload(c.t, "SessionEnd", sessionID, c.project, "", nil))
}

func newCodexTab(t *testing.T) codexTab {
	return codexTab{t: t, store: openHookTestStore(t), project: t.TempDir()}
}

func TestSessionEndRecordsNoTabSession(t *testing.T) {
	store := openHookTestStore(t)

	hookSessionEnd(store, hookEvent{SessionID: "session-a", TabID: "tab-1", Vendor: "claude"})
	hookSessionEnd(store, hookEvent{SessionID: "session-b", TabID: "tab-1", Vendor: "codex"})

	if got := tabSessionOf(t, store, "tab-1"); got != "" {
		t.Errorf("tab session = %q, want none", got)
	}
}

// /clear on 0.167: a new thread whose SessionStart arrives with its first prompt, and no SessionEnd for the thread left.
func TestCodexClearKeepsTheNewSession(t *testing.T) {
	tab := newCodexTab(t)
	tab.start("session-a", "startup")
	tab.prompt("session-a")
	tab.shell("session-a", "ls")
	if got := tabSessionOf(t, tab.store, "tab-1"); got != "session-a" {
		t.Fatalf("before /clear, tab session = %q, want session-a", got)
	}

	tab.start("session-b", "clear")
	tab.prompt("session-b")
	tab.shell("session-b", "ls")
	if got := tabSessionOf(t, tab.store, "tab-1"); got != "session-b" {
		t.Fatalf("after /clear, tab session = %q, want session-b", got)
	}

	tab.end("session-a")
	if got := tabSessionOf(t, tab.store, "tab-1"); got != "session-b" {
		t.Errorf("a late SessionEnd for the cleared session moved the tab to %q", got)
	}
}

// /resume on 0.167: a SessionEnd naming the resumed session at once, its SessionStart at the first turn, and the SessionEnd for the session left up to a minute later.
func TestCodexResumeKeepsTheResumedSession(t *testing.T) {
	tab := newCodexTab(t)
	tab.start("session-b", "startup")
	tab.prompt("session-b")

	tab.end("session-a")
	if got := tabSessionOf(t, tab.store, "tab-1"); got != "session-b" {
		t.Fatalf("SessionEnd for the resumed session moved the tab to %q before its first turn", got)
	}

	tab.start("session-a", "resume")
	tab.prompt("session-a")
	if got := tabSessionOf(t, tab.store, "tab-1"); got != "session-a" {
		t.Fatalf("after the resumed session's first turn, tab session = %q, want session-a", got)
	}

	tab.end("session-b")
	tab.shell("session-a", "ls")
	if got := tabSessionOf(t, tab.store, "tab-1"); got != "session-a" {
		t.Errorf("the late SessionEnd for the session left moved the tab to %q", got)
	}
}

func TestCodexPromptOrToolCallAloneRecordsTheSession(t *testing.T) {
	tab := newCodexTab(t)
	tab.prompt("session-a")
	if got := tabSessionOf(t, tab.store, "tab-1"); got != "session-a" {
		t.Fatalf("after a prompt, tab session = %q, want session-a", got)
	}
	tab.shell("session-b", "ls")
	if got := tabSessionOf(t, tab.store, "tab-1"); got != "session-b" {
		t.Errorf("after a tool call, tab session = %q, want session-b", got)
	}
}

func TestCodexTabNeverPromptedKeepsNoSession(t *testing.T) {
	tab := newCodexTab(t)
	tab.end("session-a")
	if got := tabSessionOf(t, tab.store, "tab-1"); got != "" {
		t.Errorf("tab session = %q, want none for a tab closed before its first prompt", got)
	}
}

func TestClaudeTabSessionComesFromSessionStartOnly(t *testing.T) {
	store := openHookTestStore(t)
	project := t.TempDir()
	hookSessionStart(store, hookEvent{SessionID: "session-a", TabID: "tab-1", Vendor: "claude"}, project)

	input, _ := json.Marshal(map[string]string{"command": "ls"})
	hookPreToolUse(store, hookEvent{SessionID: "session-sub", TabID: "tab-1", Vendor: "claude", ToolName: "Bash", ToolInput: input, Cwd: project}, project)
	hookUserPromptSubmit(store, hookEvent{SessionID: "session-sub", TabID: "tab-1", Vendor: "claude"})
	if got := tabSessionOf(t, store, "tab-1"); got != "session-a" {
		t.Fatalf("Claude tool call or prompt moved the tab to %q", got)
	}

	hookSessionStart(store, hookEvent{SessionID: "session-b", TabID: "tab-1", Vendor: "claude", SessionSource: "clear"}, project)
	if got := tabSessionOf(t, store, "tab-1"); got != "session-b" {
		t.Errorf("after a Claude /clear, tab session = %q, want session-b", got)
	}
}
