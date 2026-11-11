// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	tabCodex  = "cccccccc-4444-4444-8444-444444444444"
	tabCursor = "dddddddd-5555-4555-8555-555555555555"
	tabStale  = "eeeeeeee-6666-4666-8666-666666666666"
	tabQuiet  = "ffffffff-7777-4777-8777-777777777777"
	tabAgy    = "abababab-8888-4888-8888-888888888888"
)

// Version 2 turns every old row into a thread row whose ref is its message id, keeping its state.
func TestMigrationToVersion2KeepsLedgerRowsAndReadState(t *testing.T) {
	store := openVersion1Store(t)
	for _, stmt := range []string{
		`INSERT INTO lg_threads (id, project_id, title, created_by, created_at) VALUES ('old001', 'testproj', 'Old', '` + tabA + `', '2026-01-01T00:00:00Z')`,
		`INSERT INTO lg_messages (thread_id, tab_id, body, created_at) VALUES ('old001', '` + tabA + `', 'one', '2026-01-01T00:00:00Z')`,
		`INSERT INTO lg_messages (thread_id, tab_id, body, created_at) VALUES ('old001', '` + tabA + `', 'two', '2026-01-01T00:00:01Z')`,
		`INSERT INTO lg_notifications (project_id, thread_id, message_id, target_tab_id, kind, state, created_at, read_at) VALUES ('testproj', 'old001', 1, '` + tabB + `', 'all', 'read', '2026-01-01T00:00:00Z', '2026-01-01T00:01:00Z')`,
		`INSERT INTO lg_notifications (project_id, thread_id, message_id, target_tab_id, kind, created_at) VALUES ('testproj', 'old001', 2, '` + tabB + `', 'you', '2026-01-01T00:00:01Z')`,
	} {
		if _, err := store.db.Exec(stmt); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}
	store.Close()

	reopened, err := Open(DBPath(), testProjectID)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	var source, ref, state, readAt string
	if err := reopened.db.QueryRow(`SELECT source, ref, state, read_at FROM lg_notifications WHERE id = 1`).Scan(&source, &ref, &state, &readAt); err != nil {
		t.Fatalf("reading the migrated row: %v", err)
	}
	if source != SourceThread || ref != "1" || state != "read" || readAt != "2026-01-01T00:01:00Z" {
		t.Errorf("migrated row = %s %s %s %s, want thread 1 read with its read_at", source, ref, state, readAt)
	}
	pending, err := reopened.PendingForTab(tabB)
	if err != nil || len(pending.Threads) != 1 || pending.Threads[0].ThreadID != "old001" || pending.Threads[0].You.Count != 1 || len(pending.Events) != 0 {
		t.Fatalf("pending after migration = %+v, %v, want the one unread row for you", pending, err)
	}
	if err := reopened.MarkMessagesRead(tabB, []int64{2}); err != nil {
		t.Fatalf("MarkMessagesRead: %v", err)
	}
	if pending, _ := reopened.PendingForTab(tabB); !pending.Empty() {
		t.Errorf("pending after reading = %+v, want none", pending)
	}
}

// A tab whose last prompt or tool call was age ago.
func setTabActiveAge(t *testing.T, s *Store, tab string, age time.Duration) {
	t.Helper()
	if err := s.SetActiveTabState(tab, StateIdle); err != nil {
		t.Fatalf("SetActiveTabState: %v", err)
	}
	at := time.Now().Add(-age).UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(`UPDATE lg_tab_state SET last_active_at = ? WHERE tab_id = ?`, at, tab); err != nil {
		t.Fatalf("aging tab activity: %v", err)
	}
}

func TestBranchRowsGoOnlyToRecentlyActiveAgentTabs(t *testing.T) {
	store := openTestStore(t)
	other, err := Open(DBPath(), "otherproj")
	if err != nil {
		t.Fatalf("Open other project: %v", err)
	}
	defer other.Close()

	for tab, vendor := range map[string]string{tabA: "claude", tabCodex: "codex", tabAgy: "agy", tabCursor: "cursor", tabStale: "claude", tabQuiet: "codex"} {
		store.RegisterTab(tab, vendor)
	}
	other.RegisterTab(tabB, "claude")
	setTabActiveAge(t, store, tabA, time.Minute)
	setTabActiveAge(t, store, tabCodex, 11*time.Hour)
	setTabActiveAge(t, store, tabAgy, 2*time.Hour)
	setTabActiveAge(t, store, tabCursor, time.Minute)
	setTabActiveAge(t, store, tabStale, 13*time.Hour)
	setTabActiveAge(t, store, tabB, time.Minute)

	tabs, err := store.InsertBranchChanged(BranchChange{From: "main", To: "feature/x", Carry: CarryCarried}, time.Now())
	if err != nil {
		t.Fatalf("InsertBranchChanged: %v", err)
	}
	if !slices.Equal(tabs, []string{tabA, tabAgy, tabCodex}) {
		t.Fatalf("notified tabs = %v, want the claude, agy and codex tabs active in the last 12 hours", tabs)
	}
	for _, tab := range []string{tabCursor, tabStale, tabQuiet, tabB} {
		if pending, _ := store.SurfaceableForTab(tab); !pending.Empty() {
			t.Errorf("%s got a branch row: %+v", tab, pending)
		}
	}
	pending, _ := store.SurfaceableForTab(tabCodex)
	if len(pending.Events) != 1 || pending.Events[0].Source != SourceBranch || len(pending.Threads) != 0 {
		t.Errorf("codex surfaceable = %+v, want one branch event", pending)
	}
}

// Restoring a tab fires SessionStart and moves updated_at, but no prompt or tool call has run, so a session from long ago stays quiet.
func TestBranchRowsSkipATabThatWasOnlyRestored(t *testing.T) {
	store := openTestStore(t)
	store.RegisterTab(tabA, "claude")
	store.RegisterTab(tabCodex, "codex")
	store.SetTabState(tabA, StateIdle)
	setTabActiveAge(t, store, tabCodex, 20*time.Hour)
	store.SetTabState(tabCodex, StateIdle)

	tabs, err := store.InsertBranchChanged(BranchChange{From: "main", To: "fix-1", Carry: CarryNone}, time.Now())
	if err != nil || len(tabs) != 0 {
		t.Errorf("notified tabs = %v, %v, want none for restored tabs", tabs, err)
	}
}

func TestBranchChangedRejectsAnUnknownCarry(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.InsertBranchChanged(BranchChange{From: "a", To: "b", Carry: "maybe"}, time.Now()); err == nil {
		t.Error("an unknown carry was accepted")
	}
}

func TestBranchLineNamesTheCarry(t *testing.T) {
	for carry, want := range map[string]string{
		CarryCarried:  "[lg] branch changed from main to fix-1, uncommitted changes carried over. Check git status before you continue.",
		CarryConflict: "[lg] the human moved the branch from main to fix-1 and the carried changes conflicted, so the stash was kept. The human is resolving this conflict. Do not edit the conflicted files or run stash, merge or rebase commands unless the conflict is handed to you.",
		CarryNone:     "[lg] branch changed from main to fix-1. Check git status before you continue.",
	} {
		ref := `{"from":"main","to":"fix-1","carry":"` + carry + `"}`
		if got := formatEvent(PendingEvent{Source: SourceBranch, Ref: ref}); got != want {
			t.Errorf("%s line = %q, want %q", carry, got, want)
		}
	}
}

// A typed nudge carries only the thread line. The branch row is left for the hook, which surfaces it at the next prompt or tool call.
func TestBranchRowsAreNeverTypedOrWake(t *testing.T) {
	store := openTestStore(t)
	g := newTestGroup(t, store)
	if _, err := store.PostMessage(tabA, g.ThreadID, "hello"); err != nil {
		t.Fatalf("PostMessage: %v", err)
	}
	store.RegisterTab(tabB, "claude")
	store.RegisterTab(tabCodex, "codex")
	setTabActiveAge(t, store, tabB, time.Minute)
	setTabActiveAge(t, store, tabCodex, time.Minute)
	if _, err := store.InsertBranchChanged(BranchChange{From: "main", To: "fix-1", Carry: CarryNone}, time.Now()); err != nil {
		t.Fatalf("InsertBranchChanged: %v", err)
	}

	if waking, _ := store.PendingTabs(); slices.Contains(waking, tabCodex) {
		t.Errorf("a tab owed only a branch row would be woken: %v", waking)
	}
	if text, ok := store.ConsumePending(tabCodex); ok {
		t.Errorf("a branch row alone was typed: %q", text)
	}
	text, ok := store.ConsumePending(tabB)
	if !ok || strings.Contains(text, "branch") || !strings.HasPrefix(text, "[lg] thread "+g.ThreadID) {
		t.Fatalf("ConsumePending = %q, %v, want the thread line only", text, ok)
	}
	for _, tab := range []string{tabB, tabCodex} {
		surfaced, _ := store.SurfaceableForTab(tab)
		if len(surfaced.Events) != 1 || formatEvent(surfaced.Events[0]) != "[lg] branch changed from main to fix-1. Check git status before you continue." {
			t.Errorf("%s surfaceable = %+v, want the branch row left for the hook", tab, surfaced)
		}
	}
}

func insertMove(t *testing.T, s *Store, from, to, carry string) {
	t.Helper()
	if _, err := s.InsertBranchChanged(BranchChange{From: from, To: to, Carry: carry}, time.Now()); err != nil {
		t.Fatalf("InsertBranchChanged: %v", err)
	}
}

// Moves the tab has not seen yet read as one line: the first from, the last to, the worst carry. Other events keep their own lines.
func TestPendingBranchMovesCombineIntoOneLine(t *testing.T) {
	store := openTestStore(t)
	store.RegisterTab(tabCodex, "codex")
	setTabActiveAge(t, store, tabCodex, time.Minute)
	insertMove(t, store, "main", "staging", CarryNone)
	if _, err := store.InsertAskAnswered(tabCodex, "ask1"); err != nil {
		t.Fatalf("InsertAskAnswered: %v", err)
	}
	insertMove(t, store, "staging", "merge-x", CarryCarried)
	insertMove(t, store, "merge-x", "release", CarryNone)

	surfaced, _ := store.SurfaceableForTab(tabCodex)
	lines := strings.Split(store.NotificationText(surfaced), "\n")
	if len(lines) != 2 || lines[0] != "[lg] branch changed from main to release, uncommitted changes carried over. Check git status before you continue." || !strings.Contains(lines[1], "ask1") {
		t.Errorf("lines = %q, want one combined branch line and the ask line", lines)
	}
}

// A wrong switch and its undo leave the tab where it was, so they say nothing, unless one of them left a conflict.
func TestBranchRoundTripSaysNothingUnlessItConflicted(t *testing.T) {
	store := openTestStore(t)
	store.RegisterTab(tabCodex, "codex")
	setTabActiveAge(t, store, tabCodex, time.Minute)
	insertMove(t, store, "feature", "staging", CarryCarried)
	insertMove(t, store, "staging", "feature", CarryCarried)
	surfaced, _ := store.SurfaceableForTab(tabCodex)
	if len(surfaced.Events) != 2 {
		t.Fatalf("surfaceable = %+v, want both rows", surfaced)
	}
	if text := store.NotificationText(surfaced); text != "" {
		t.Errorf("round trip text = %q, want nothing", text)
	}
	store.MarkNotificationsSent(surfaced.RowIDs())

	insertMove(t, store, "feature", "staging", CarryConflict)
	insertMove(t, store, "staging", "feature", CarryNone)
	surfaced, _ = store.SurfaceableForTab(tabCodex)
	if text := store.NotificationText(surfaced); !strings.HasPrefix(text, "[lg] the human moved the branch from feature to feature and the carried changes conflicted") {
		t.Errorf("conflicting round trip text = %q, want the conflict line", text)
	}
}

// The tab handed the open conflict gets no branch rows while it is open, and the hand-off settles the ones it had not seen.
func TestConflictAssigneeGetsNoBranchNotice(t *testing.T) {
	store := openTestStore(t)
	store.RegisterTab(tabA, "claude")
	store.RegisterTab(tabCodex, "codex")
	setTabActiveAge(t, store, tabA, time.Minute)
	setTabActiveAge(t, store, tabCodex, time.Minute)
	insertMove(t, store, "feature", "merge-x", CarryNone)

	if _, err := store.OpenConflict(conflictScope, tabA, ""); err != nil {
		t.Fatalf("OpenConflict: %v", err)
	}
	if surfaced, _ := store.SurfaceableForTab(tabA); !surfaced.Empty() {
		t.Errorf("the assignee kept a branch row past the hand-off: %+v", surfaced)
	}
	if surfaced, _ := store.SurfaceableForTab(tabCodex); len(surfaced.Events) != 1 {
		t.Errorf("another tab lost its branch row at the hand-off: %+v", surfaced)
	}

	tabs, err := store.InsertBranchChanged(BranchChange{From: "merge-x", To: "staging", Carry: CarryConflict}, time.Now())
	if err != nil || !slices.Equal(tabs, []string{tabCodex}) {
		t.Errorf("notified tabs = %v, %v, want only the tab not handed the conflict", tabs, err)
	}

	if _, err := store.CloseConflict(conflictScope, ""); err != nil {
		t.Fatalf("CloseConflict: %v", err)
	}
	tabs, _ = store.InsertBranchChanged(BranchChange{From: "staging", To: "main", Carry: CarryNone}, time.Now())
	if !slices.Contains(tabs, tabA) {
		t.Errorf("notified tabs after the conflict closed = %v, want the former assignee back", tabs)
	}
}
