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
	store := openTestStore(t)
	store.RegisterTab(notifyTabAuthor, "claude")
	store.RegisterTab(notifyTabTarget, "codex")
	if _, err := store.CreateThread(notifyTabAuthor, "Review", "hi !>>"+notifyTabTarget+"<<!"); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	return store
}

func TestListenOnceWritesHeartbeatAndConsumesPending(t *testing.T) {
	store := storeWithPendingMention(t)

	if text, ok := listenOnce(store, notifyTabAuthor); ok || text != "" {
		t.Errorf("listenOnce for a tab with nothing pending = %q, %v", text, ok)
	}
	text, ok := listenOnce(store, notifyTabTarget)
	if !ok || !strings.Contains(text, ": 1 for you") {
		t.Fatalf("listenOnce = %q, %v, want the nudge", text, ok)
	}
	if _, ok := listenOnce(store, notifyTabTarget); ok {
		t.Error("listenOnce returned the same notification twice")
	}
}
