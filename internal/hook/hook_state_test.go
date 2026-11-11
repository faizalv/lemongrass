// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/session"
)

func stateOf(t *testing.T, store *session.Store, tab string) string {
	t.Helper()
	state, err := store.TabState(tab)
	if err != nil {
		t.Fatalf("TabState: %v", err)
	}
	return state
}

func TestTabStateFollowsTheAgentsHookEvents(t *testing.T) {
	store := openHookTestStore(t)
	tab := notifyTabTarget
	steps := []struct {
		event   string
		payload hookEvent
		want    string
	}{
		{"SessionStart", hookEvent{}, session.StateIdle},
		{"UserPromptSubmit", hookEvent{}, session.StateWorking},
		{"PreToolUse", hookEvent{}, session.StateWorking},
		{"Notification", hookEvent{NotificationType: "permission_prompt"}, session.StatePrompting},
		{"PostToolUse", hookEvent{}, session.StateWorking},
		{"Stop", hookEvent{}, session.StateIdle},
		{"UserPromptSubmit", hookEvent{}, session.StateWorking},
		{"PermissionRequest", hookEvent{}, session.StatePrompting},
		{"PostToolUse", hookEvent{}, session.StateWorking},
		{"Stop", hookEvent{}, session.StateIdle},
		{"UserPromptSubmit", hookEvent{}, session.StateWorking},
		{"PermissionRequest", hookEvent{}, session.StatePrompting},
		{"Interrupt", hookEvent{}, session.StateIdle},
		{"Notification", hookEvent{Message: "Claude needs your permission to use Bash"}, session.StatePrompting},
		{"Notification", hookEvent{NotificationType: "idle_prompt"}, session.StateIdle},
		{"Notification", hookEvent{NotificationType: "auth_success"}, session.StateIdle},
		{"PreToolUse", hookEvent{ToolName: "AskUserQuestion"}, session.StatePrompting},
		{"PostToolUse", hookEvent{ToolName: "AskUserQuestion"}, session.StateWorking},
		{"PreToolUse", hookEvent{ToolName: "Bash"}, session.StateWorking},
	}
	for _, step := range steps {
		step.payload.TabID = tab
		recordTabState(store, step.event, step.payload)
		if got := stateOf(t, store, tab); got != step.want {
			t.Fatalf("after %s the state is %s, want %s", step.event, got, step.want)
		}
	}

	recordTabState(store, "PreToolUse", hookEvent{TabID: tab})
	recordTabState(store, "SessionEnd", hookEvent{TabID: tab})
	if got := stateOf(t, store, tab); got != "" {
		t.Errorf("after SessionEnd the state is %s, want it cleared", got)
	}
}

func TestTabStateIgnoresSessionsWithoutATab(t *testing.T) {
	store := openHookTestStore(t)
	recordTabState(store, "PreToolUse", hookEvent{})
	if got := stateOf(t, store, notifyTabTarget); got != "" {
		t.Errorf("a hook with no tab changed a tab's state to %s", got)
	}
}

func TestConsumePendingReturnsTheNudgeOnceAndMarksItSent(t *testing.T) {
	store := storeWithPendingMention(t)
	text, ok := store.ConsumePending(notifyTabTarget)
	if !ok || !strings.HasPrefix(text, "[lg] thread ") || !strings.Contains(text, ": 1 for you from aaaaaaaa") || strings.Contains(text, "Review") {
		t.Fatalf("ConsumePending = %q, %v, want a title-free nudge", text, ok)
	}
	if again, ok := store.ConsumePending(notifyTabTarget); ok {
		t.Errorf("a second call returned %q, want nothing pending", again)
	}
}

// Session start, stop, idle and permission prompts keep the tab's last activity; only a prompt or a tool call moves it.
func TestOnlyPromptsAndToolCallsRecordActivity(t *testing.T) {
	store := openHookTestStore(t)
	tab := notifyTabTarget
	active := func() bool {
		_, ok, err := store.TabLastActive(tab)
		if err != nil {
			t.Fatalf("TabLastActive: %v", err)
		}
		return ok
	}

	for _, quiet := range []struct {
		event   string
		payload hookEvent
	}{
		{"SessionStart", hookEvent{}},
		{"Stop", hookEvent{}},
		{"Interrupt", hookEvent{}},
		{"PermissionRequest", hookEvent{}},
		{"Notification", hookEvent{NotificationType: "idle_prompt"}},
		{"Notification", hookEvent{NotificationType: "permission_prompt"}},
	} {
		quiet.payload.TabID = tab
		recordTabState(store, quiet.event, quiet.payload)
		if active() {
			t.Fatalf("%s recorded activity", quiet.event)
		}
	}

	for _, event := range []string{"UserPromptSubmit", "PreToolUse", "PostToolUse"} {
		recordTabState(store, "SessionEnd", hookEvent{TabID: tab})
		recordTabState(store, event, hookEvent{TabID: tab})
		if !active() {
			t.Errorf("%s did not record activity", event)
		}
	}
}
