// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"strings"
	"testing"
	"time"
)

func seedTabs(store *Store) {
	store.RegisterTab(tabA, "claude")
	store.RegisterTab(tabB, "codex")
}

func TestMentionCreatesPendingRowForTargetOnly(t *testing.T) {
	store := openTestStore(t)
	seedTabs(store)

	id, err := store.CreateThread(tabA, "Review", "hey !>>"+tabB+"<<! and me !>>"+tabA+"<<!")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	pending, err := threadsOf(store.PendingForTab(tabB))
	if err != nil || len(pending) != 1 {
		t.Fatalf("PendingForTab(B) = %+v, %v, want one thread", pending, err)
	}
	p := pending[0]
	if p.ThreadID != id || p.Title != "Review" || p.Count() != 1 || p.You.Count != 1 || len(p.You.Tabs) != 1 || p.You.Tabs[0] != tabA {
		t.Errorf("pending = %+v, want thread %s, one row, sender tabA", p, id)
	}
	if own, _ := threadsOf(store.PendingForTab(tabA)); len(own) != 0 {
		t.Errorf("the author was notified of their own mention: %+v", own)
	}
}

func TestPendingCoalescesPerThreadWithDistinctSenders(t *testing.T) {
	store := openTestStore(t)
	seedTabs(store)
	store.RegisterTab(tabC, "codex")

	id, _ := store.CreateThread(tabA, "Review", "one !>>"+tabB+"<<!")
	store.PostMessage(tabA, id, "two !>>"+tabB+"<<!")
	store.PostMessage(tabC, id, "three !>>"+tabB+"<<!")
	other, _ := store.CreateThread(tabA, "Other", "x !>>"+tabB+"<<!")

	pending, _ := threadsOf(store.PendingForTab(tabB))
	if len(pending) != 2 || pending[0].ThreadID != id || pending[1].ThreadID != other {
		t.Fatalf("pending = %+v, want threads %s then %s", pending, id, other)
	}
	if pending[0].Count() != 3 || len(pending[0].You.Tabs) != 2 {
		t.Errorf("first thread: %d rows, %d senders, want 3 rows and 2 distinct senders", pending[0].Count(), len(pending[0].You.Tabs))
	}
}

func TestMarkSentRemovesRowsFromPending(t *testing.T) {
	store := openTestStore(t)
	seedTabs(store)
	store.CreateThread(tabA, "Review", "x !>>"+tabB+"<<!")

	pending, _ := threadsOf(store.PendingForTab(tabB))
	if err := store.MarkNotificationsSent(pending[0].RowIDs); err != nil {
		t.Fatalf("MarkNotificationsSent: %v", err)
	}
	if again, _ := threadsOf(store.PendingForTab(tabB)); len(again) != 0 {
		t.Errorf("sent rows still pending: %+v", again)
	}
	var state string
	store.db.QueryRow(`SELECT state FROM lg_notifications LIMIT 1`).Scan(&state)
	if state != "sent" {
		t.Errorf("state = %q, want sent", state)
	}
}

func TestReadingMessagesSettlesOnlyThoseRowsAndKeepsThem(t *testing.T) {
	store := openTestStore(t)
	seedTabs(store)
	id, _ := store.CreateThread(tabA, "Review", "one !>>"+tabB+"<<!")
	second, _ := store.PostMessage(tabA, id, "two !>>"+tabB+"<<!")

	msgs, _, _ := store.ReadThread(id, second.ID, 10)
	var ids []int64
	for _, m := range msgs {
		ids = append(ids, m.ID)
	}
	if err := store.MarkMessagesRead(tabB, ids); err != nil {
		t.Fatalf("MarkMessagesRead: %v", err)
	}

	pending, _ := threadsOf(store.PendingForTab(tabB))
	if len(pending) != 1 || pending[0].Count() != 1 {
		t.Errorf("pending = %+v, want the one unread row", pending)
	}
	var total, read int
	store.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(state = 'read'), 0) FROM lg_notifications`).Scan(&total, &read)
	if total != 2 || read != 1 {
		t.Errorf("rows total %d read %d, want 2 kept and 1 read", total, read)
	}
}

// Moves the tab's typed rows back past the retype window, as if their line went out that long ago.
func ageTypedRows(t *testing.T, store *Store, tab string) {
	t.Helper()
	past := time.Now().Add(-RetypeAfter - time.Second).UTC().Format(time.RFC3339Nano)
	if _, err := store.db.Exec(`UPDATE lg_notifications SET sent_at = ? WHERE target_tab_id = ? AND state = 'typed'`, past, tab); err != nil {
		t.Fatalf("aging typed rows: %v", err)
	}
}

func TestATypedMessageStaysOwedUntilReadUpToTheCap(t *testing.T) {
	store := openTestStore(t)
	seedTabs(store)
	id, _ := store.CreateThread(tabA, "Review", "x !>>"+tabB+"<<!")

	if _, ok := store.ConsumePending(tabB); !ok {
		t.Fatal("nothing composed for the mention")
	}
	if _, ok := store.ConsumePending(tabB); ok {
		t.Error("a line just typed was composed again inside the retype window")
	}
	if tabs, _ := store.PendingTabs(); len(tabs) != 0 {
		t.Errorf("PendingTabs = %v, want none inside the retype window", tabs)
	}
	if surfaced, _ := store.SurfaceableForTab(tabB); !surfaced.Empty() {
		t.Errorf("a typed row was surfaced as hook context: %+v", surfaced)
	}

	for typing := 2; typing <= MaxTypings; typing++ {
		ageTypedRows(t, store, tabB)
		if tabs, _ := store.PendingTabs(); len(tabs) != 1 || tabs[0] != tabB {
			t.Fatalf("typing %d: PendingTabs = %v, want B owed again", typing, tabs)
		}
		if _, ok := store.ConsumePending(tabB); !ok {
			t.Fatalf("typing %d: an unread typed row was not composed again", typing)
		}
	}
	ageTypedRows(t, store, tabB)
	if _, ok := store.ConsumePending(tabB); ok {
		t.Errorf("composed past %d typings", MaxTypings)
	}

	msgs, _, _ := store.ReadThread(id, 0, 10)
	store.MarkMessagesRead(tabB, []int64{msgs[0].ID})
	var state string
	store.db.QueryRow(`SELECT state FROM lg_notifications WHERE target_tab_id = ?`, tabB).Scan(&state)
	if state != "read" {
		t.Errorf("state after reading = %q, want read", state)
	}
}

func TestAReadAfterTypingEndsTheRetypes(t *testing.T) {
	store := openTestStore(t)
	seedTabs(store)
	id, _ := store.CreateThread(tabA, "Review", "x !>>"+tabB+"<<!")
	store.ConsumePending(tabB)
	msgs, _, _ := store.ReadThread(id, 0, 10)
	store.MarkMessagesRead(tabB, []int64{msgs[0].ID})
	ageTypedRows(t, store, tabB)
	if tabs, _ := store.PendingTabs(); len(tabs) != 0 {
		t.Errorf("PendingTabs = %v, want nothing owed once read", tabs)
	}
}

func TestPruneNotificationsKeepsThirtyDays(t *testing.T) {
	store := openTestStore(t)
	seedTabs(store)
	store.CreateThread(tabA, "old", "x !>>"+tabB+"<<!")
	store.CreateThread(tabA, "new", "y !>>"+tabB+"<<!")
	old := time.Now().Add(-NotificationRetention - time.Hour).UTC().Format(time.RFC3339Nano)
	store.db.Exec(`UPDATE lg_notifications SET created_at = ?, state = 'read' WHERE id = 1`, old)

	if err := store.PruneNotifications(time.Now()); err != nil {
		t.Fatalf("PruneNotifications: %v", err)
	}
	var count int
	store.db.QueryRow(`SELECT COUNT(*) FROM lg_notifications`).Scan(&count)
	if count != 1 {
		t.Errorf("rows after prune = %d, want only the recent one", count)
	}
}

func TestVendorForTabFallsBackToGroupMembership(t *testing.T) {
	store := openTestStore(t)
	store.RegisterTab(tabA, "claude")
	store.db.Exec(`INSERT INTO lg_groups (id, project_id, leader_tab_id, created_at) VALUES (1, ?, ?, ?)`, testProjectID, tabA, now())
	store.db.Exec(`INSERT INTO lg_group_members (group_id, tab_id, role, label, vendor) VALUES (1, ?, 'thinker', 'r', 'codex')`, tabB)

	if v, _ := store.VendorForTab(tabA); v != "claude" {
		t.Errorf("VendorForTab(A) = %q, want claude", v)
	}
	if v, _ := store.VendorForTab(tabB); v != "codex" {
		t.Errorf("VendorForTab(B) = %q, want the group member's codex", v)
	}
	if v, _ := store.VendorForTab("nobody"); v != "" {
		t.Errorf("VendorForTab(unknown) = %q, want empty", v)
	}
}

func TestFormatNotificationCoalescesAndCarriesNoContent(t *testing.T) {
	out := FormatNotification([]PendingThread{
		{ThreadID: "t4", Title: "Review", All: KindPart{Count: 2, Tabs: []string{tabA, tabB}}, RowIDs: []int64{1, 2}},
		{ThreadID: "t5", Title: "Other", All: KindPart{Count: 1, Tabs: []string{tabB}}, RowIDs: []int64{3}},
	}, nil)
	for _, want := range []string{"[lg] thread t4: 2 new from aaaaaaaa, bbbbbbbb", "[lg] thread t5: 1 new from bbbbbbbb"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// The thread part of a pending read, for tests that post only to threads.
func threadsOf(p Pending, err error) ([]PendingThread, error) {
	return p.Threads, err
}
