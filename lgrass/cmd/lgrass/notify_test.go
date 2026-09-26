package main

import (
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/session"
)

const (
	notifyTabAuthor = "aaaaaaaa-1111-4111-8111-111111111111"
	notifyTabTarget = "bbbbbbbb-2222-4222-8222-222222222222"
)

func storeWithPendingMention(t *testing.T) *session.Store {
	t.Helper()
	store := openHookTestStore(t)
	store.RegisterTab(notifyTabAuthor, "claude")
	store.RegisterTab(notifyTabTarget, "codex")
	if _, err := store.CreateThread(notifyTabAuthor, "Review", "hi !>>"+notifyTabTarget+"<<!"); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	return store
}

func TestNotificationContextSurfacesOnceThenMarksSent(t *testing.T) {
	store := storeWithPendingMention(t)

	first := notificationContext(store, notifyTabTarget)
	if len(first) != 1 || !strings.Contains(first[0], "1 new message in thread 1 [Review]") || strings.Contains(first[0], "hi !>>") {
		t.Fatalf("first = %q, want one content-free nudge", first)
	}
	if again := notificationContext(store, notifyTabTarget); len(again) != 0 {
		t.Errorf("second call repeated the nudge: %q", again)
	}
}

func TestNotificationContextIgnoresTabsWithNothingOrNoTab(t *testing.T) {
	store := storeWithPendingMention(t)

	if got := notificationContext(store, notifyTabAuthor); len(got) != 0 {
		t.Errorf("the author was nudged: %q", got)
	}
	if got := notificationContext(store, ""); len(got) != 0 {
		t.Errorf("a call with no tab id was nudged: %q", got)
	}
}

func TestPostToolUseAndSessionStartCarryTheNudge(t *testing.T) {
	store := storeWithPendingMention(t)

	result := hookPostToolUse(store, hookEvent{SessionID: "s", ToolName: "Bash", TabID: notifyTabTarget}, t.TempDir())
	if !strings.Contains(result.AdditionalContext, "new message in thread 1") {
		t.Errorf("PostToolUse context = %q, want the nudge", result.AdditionalContext)
	}

	store.CreateThread(notifyTabAuthor, "Second", "again !>>"+notifyTabTarget+"<<!")
	start := hookSessionStart(store, hookEvent{SessionID: "s2", TabID: notifyTabTarget}, t.TempDir())
	if !strings.Contains(start.AdditionalContext, "thread 2 [Second]") {
		t.Errorf("SessionStart context = %q, want the pending nudge", start.AdditionalContext)
	}
}

func TestListenOnceWritesHeartbeatAndConsumesPending(t *testing.T) {
	store := storeWithPendingMention(t)

	if text, ok := listenOnce(store, notifyTabAuthor); ok || text != "" {
		t.Errorf("listenOnce for a tab with nothing pending = %q, %v", text, ok)
	}
	text, ok := listenOnce(store, notifyTabTarget)
	if !ok || !strings.Contains(text, "thread 1 [Review]") {
		t.Fatalf("listenOnce = %q, %v, want the nudge", text, ok)
	}
	if _, ok := listenOnce(store, notifyTabTarget); ok {
		t.Error("listenOnce returned the same notification twice")
	}
}
