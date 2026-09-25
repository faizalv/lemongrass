package session

import (
	"testing"
	"time"
)

const testProjectID = "testproj"

func openTestStore(t *testing.T) *Store {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	store, err := Open(DBPath(), testProjectID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestLivenessExcludesSelfAndReportsActiveIdling(t *testing.T) {
	store := openTestStore(t)

	if err := store.Start("session-a", "", ""); err != nil {
		t.Fatalf("Start a: %v", err)
	}
	if err := store.Start("session-b", "", ""); err != nil {
		t.Fatalf("Start b: %v", err)
	}

	// Backdate session-b's activity so it reads as idling.
	old := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	if _, err := store.db.Exec(
		`UPDATE sessions SET last_activity_at = ? WHERE project_id = ? AND session_id = ?`,
		old, testProjectID, "session-b",
	); err != nil {
		t.Fatalf("backdating session-b: %v", err)
	}

	liveness, err := store.Liveness("session-a", 5*time.Minute)
	if err != nil {
		t.Fatalf("Liveness: %v", err)
	}
	if len(liveness) != 1 {
		t.Fatalf("len(liveness) = %d, want 1 (session-a excluded as self)", len(liveness))
	}
	if liveness[0].SessionID != "session-b" {
		t.Errorf("liveness[0].SessionID = %q, want session-b", liveness[0].SessionID)
	}
	if liveness[0].Active {
		t.Error("session-b backdated 1h with a 5m threshold should read as idling, got Active=true")
	}
}

func TestEndedSessionExcludedFromLiveness(t *testing.T) {
	store := openTestStore(t)

	if err := store.Start("session-a", "", ""); err != nil {
		t.Fatalf("Start a: %v", err)
	}
	if err := store.Start("session-b", "", ""); err != nil {
		t.Fatalf("Start b: %v", err)
	}
	if err := store.End("session-b"); err != nil {
		t.Fatalf("End b: %v", err)
	}

	liveness, err := store.Liveness("session-a", 5*time.Minute)
	if err != nil {
		t.Fatalf("Liveness: %v", err)
	}
	if len(liveness) != 0 {
		t.Fatalf("len(liveness) = %d, want 0 (session-b has ended)", len(liveness))
	}
}

func TestRecentActivitySameFileAndSameFolder(t *testing.T) {
	store := openTestStore(t)

	if err := store.Start("session-a", "", ""); err != nil {
		t.Fatalf("Start a: %v", err)
	}
	if err := store.Start("session-b", "", ""); err != nil {
		t.Fatalf("Start b: %v", err)
	}
	if err := store.LogFileActivity("session-b", "/proj/pkg/foo.go"); err != nil {
		t.Fatalf("LogFileActivity: %v", err)
	}

	t.Run("same file", func(t *testing.T) {
		hits, err := store.RecentActivity("session-a", "/proj/pkg/foo.go", 15*time.Minute)
		if err != nil {
			t.Fatalf("RecentActivity: %v", err)
		}
		if len(hits) != 1 || hits[0].SessionID != "session-b" || !hits[0].SameFile {
			t.Fatalf("hits = %+v, want one same-file hit from session-b", hits)
		}
	})

	t.Run("same folder, different file", func(t *testing.T) {
		hits, err := store.RecentActivity("session-a", "/proj/pkg/bar.go", 15*time.Minute)
		if err != nil {
			t.Fatalf("RecentActivity: %v", err)
		}
		if len(hits) != 1 || hits[0].SessionID != "session-b" || hits[0].SameFile {
			t.Fatalf("hits = %+v, want one same-folder (not same-file) hit from session-b", hits)
		}
	})

	t.Run("different folder", func(t *testing.T) {
		hits, err := store.RecentActivity("session-a", "/proj/other/baz.go", 15*time.Minute)
		if err != nil {
			t.Fatalf("RecentActivity: %v", err)
		}
		if len(hits) != 0 {
			t.Fatalf("hits = %+v, want none (different folder)", hits)
		}
	})

	t.Run("excludes the querying session's own activity", func(t *testing.T) {
		if err := store.LogFileActivity("session-a", "/proj/pkg/foo.go"); err != nil {
			t.Fatalf("LogFileActivity: %v", err)
		}
		hits, err := store.RecentActivity("session-a", "/proj/pkg/foo.go", 15*time.Minute)
		if err != nil {
			t.Fatalf("RecentActivity: %v", err)
		}
		for _, h := range hits {
			if h.SessionID == "session-a" {
				t.Fatalf("RecentActivity leaked the querying session's own activity: %+v", hits)
			}
		}
	})
}

func TestRecentActivityExcludesStaleAndEndedSessions(t *testing.T) {
	store := openTestStore(t)

	if err := store.Start("session-a", "", ""); err != nil {
		t.Fatalf("Start a: %v", err)
	}
	if err := store.Start("session-b", "", ""); err != nil {
		t.Fatalf("Start b: %v", err)
	}
	if err := store.LogFileActivity("session-b", "/proj/pkg/foo.go"); err != nil {
		t.Fatalf("LogFileActivity: %v", err)
	}

	old := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	if _, err := store.db.Exec(
		`UPDATE file_activity SET touched_at = ? WHERE project_id = ? AND session_id = ?`,
		old, testProjectID, "session-b",
	); err != nil {
		t.Fatalf("backdating activity: %v", err)
	}

	hits, err := store.RecentActivity("session-a", "/proj/pkg/foo.go", 15*time.Minute)
	if err != nil {
		t.Fatalf("RecentActivity: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("hits = %+v, want none (activity is stale)", hits)
	}

	// Fresh activity, but the session has since ended and is no longer "live".
	if err := store.LogFileActivity("session-b", "/proj/pkg/foo.go"); err != nil {
		t.Fatalf("LogFileActivity: %v", err)
	}
	if err := store.End("session-b"); err != nil {
		t.Fatalf("End b: %v", err)
	}
	hits, err = store.RecentActivity("session-a", "/proj/pkg/foo.go", 15*time.Minute)
	if err != nil {
		t.Fatalf("RecentActivity: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("hits = %+v, want none (session-b has ended)", hits)
	}
}

func TestIncrementNudgeCounterFiresAtThresholdThenResets(t *testing.T) {
	store := openTestStore(t)
	if err := store.Start("session-a", "", ""); err != nil {
		t.Fatalf("Start: %v", err)
	}

	const threshold = 8
	for i := 1; i < threshold; i++ {
		fire, err := store.IncrementNudgeCounter("session-a", threshold)
		if err != nil {
			t.Fatalf("IncrementNudgeCounter call %d: %v", i, err)
		}
		if fire {
			t.Fatalf("fired early on call %d, want fire only on call %d", i, threshold)
		}
	}

	fire, err := store.IncrementNudgeCounter("session-a", threshold)
	if err != nil {
		t.Fatalf("IncrementNudgeCounter call %d: %v", threshold, err)
	}
	if !fire {
		t.Fatalf("did not fire on call %d", threshold)
	}

	// Counter should have reset, so another threshold-1 calls shouldn't fire again.
	for i := 1; i < threshold; i++ {
		fire, err := store.IncrementNudgeCounter("session-a", threshold)
		if err != nil {
			t.Fatalf("post-reset call %d: %v", i, err)
		}
		if fire {
			t.Fatalf("fired again too early on post-reset call %d", i)
		}
	}
}

func TestPostThreadMessageAndRecentThreadMessages(t *testing.T) {
	store := openTestStore(t)

	if _, err := store.PostThreadMessage("session-a", "first", ""); err != nil {
		t.Fatalf("PostThreadMessage: %v", err)
	}
	if _, err := store.PostThreadMessage("session-b", "second", "session-a"); err != nil {
		t.Fatalf("PostThreadMessage: %v", err)
	}

	msgs, err := store.RecentThreadMessages(10)
	if err != nil {
		t.Fatalf("RecentThreadMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2", len(msgs))
	}
	// Newest first.
	if msgs[0].Body != "second" || msgs[0].Mention != "session-a" {
		t.Errorf("msgs[0] = %+v, want the second, mentioning session-a", msgs[0])
	}
	if msgs[1].Body != "first" {
		t.Errorf("msgs[1] = %+v, want the first message", msgs[1])
	}
}

func TestRecentThreadMessagesRespectsLimit(t *testing.T) {
	store := openTestStore(t)
	for i := 0; i < 5; i++ {
		if _, err := store.PostThreadMessage("session-a", "msg", ""); err != nil {
			t.Fatalf("PostThreadMessage: %v", err)
		}
	}
	msgs, err := store.RecentThreadMessages(2)
	if err != nil {
		t.Fatalf("RecentThreadMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2 (limit)", len(msgs))
	}
}

func TestUnreadMentionsOnlyReturnsUnreadMentionsAddressedToSelf(t *testing.T) {
	store := openTestStore(t)
	if err := store.Start("session-a", "", ""); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if _, err := store.PostThreadMessage("session-b", "not a mention", ""); err != nil {
		t.Fatalf("PostThreadMessage: %v", err)
	}
	if _, err := store.PostThreadMessage("session-b", "mentions someone else", "session-c"); err != nil {
		t.Fatalf("PostThreadMessage: %v", err)
	}
	if _, err := store.PostThreadMessage("session-b", "mentions session-a", "session-a"); err != nil {
		t.Fatalf("PostThreadMessage: %v", err)
	}

	msgs, err := store.UnreadMentions("session-a")
	if err != nil {
		t.Fatalf("UnreadMentions: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Body != "mentions session-a" {
		t.Fatalf("UnreadMentions = %+v, want exactly the one message mentioning session-a", msgs)
	}

	if err := store.MarkThreadRead("session-a"); err != nil {
		t.Fatalf("MarkThreadRead: %v", err)
	}
	msgs, err = store.UnreadMentions("session-a")
	if err != nil {
		t.Fatalf("UnreadMentions after mark-read: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("UnreadMentions after mark-read = %+v, want none (already surfaced once)", msgs)
	}
}

func TestUnreadMentionsExcludesSelfMention(t *testing.T) {
	store := openTestStore(t)
	if err := store.Start("session-a", "", ""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := store.PostThreadMessage("session-a", "mentioning myself", "session-a"); err != nil {
		t.Fatalf("PostThreadMessage: %v", err)
	}
	msgs, err := store.UnreadMentions("session-a")
	if err != nil {
		t.Fatalf("UnreadMentions: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("UnreadMentions = %+v, want none (a session's own post never surfaces to itself)", msgs)
	}
}

func TestLiveMessagingTargetsExcludesSelfEndedAndSocketless(t *testing.T) {
	store := openTestStore(t)
	if err := store.Start("session-a", "/tmp/a.sock", "token-a"); err != nil {
		t.Fatalf("Start a: %v", err)
	}
	if err := store.Start("session-b", "/tmp/b.sock", "token-b"); err != nil {
		t.Fatalf("Start b: %v", err)
	}
	if err := store.Start("session-c", "", ""); err != nil {
		t.Fatalf("Start c (no socket): %v", err)
	}
	if err := store.Start("session-d", "/tmp/d.sock", "token-d"); err != nil {
		t.Fatalf("Start d: %v", err)
	}
	if err := store.End("session-d"); err != nil {
		t.Fatalf("End d: %v", err)
	}

	targets, err := store.LiveMessagingTargets("session-a")
	if err != nil {
		t.Fatalf("LiveMessagingTargets: %v", err)
	}
	if len(targets) != 1 || targets[0].SessionID != "session-b" || targets[0].Socket != "/tmp/b.sock" || targets[0].Token != "token-b" {
		t.Fatalf("targets = %+v, want exactly session-b (a is self, c has no socket, d has ended)", targets)
	}
}

func TestLatestThreadMessageIDEmptyIsZero(t *testing.T) {
	store := openTestStore(t)
	id, err := store.LatestThreadMessageID()
	if err != nil {
		t.Fatalf("LatestThreadMessageID: %v", err)
	}
	if id != 0 {
		t.Errorf("LatestThreadMessageID on an empty project = %d, want 0", id)
	}
}

func TestNewThreadMessagesAfterOnlyReturnsLaterMessages(t *testing.T) {
	store := openTestStore(t)

	if _, err := store.PostThreadMessage("session-a", "before listening started", ""); err != nil {
		t.Fatalf("PostThreadMessage: %v", err)
	}
	cursor, err := store.LatestThreadMessageID()
	if err != nil {
		t.Fatalf("LatestThreadMessageID: %v", err)
	}

	if _, err := store.PostThreadMessage("session-b", "first new", ""); err != nil {
		t.Fatalf("PostThreadMessage: %v", err)
	}
	if _, err := store.PostThreadMessage("session-c", "second new", ""); err != nil {
		t.Fatalf("PostThreadMessage: %v", err)
	}

	msgs, err := store.NewThreadMessagesAfter(cursor)
	if err != nil {
		t.Fatalf("NewThreadMessagesAfter: %v", err)
	}
	if len(msgs) != 2 || msgs[0].Body != "first new" || msgs[1].Body != "second new" {
		t.Fatalf("NewThreadMessagesAfter = %+v, want the two later messages in order, not the earlier one", msgs)
	}
}

func TestIncrementNudgeCounterWithoutPriorStart(t *testing.T) {
	store := openTestStore(t)
	// No Start call. The defensive upsert path should still work.
	fire, err := store.IncrementNudgeCounter("session-a", 1)
	if err != nil {
		t.Fatalf("IncrementNudgeCounter: %v", err)
	}
	if !fire {
		t.Error("threshold of 1 should fire on the first call even without a prior Start")
	}
}

func TestEnsureOpenPreservesExistingSignature(t *testing.T) {
	store := openTestStore(t)
	if err := store.Start("session-a", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Sign("session-a", "checklist-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.End("session-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureOpen("session-a"); err != nil {
		t.Fatal(err)
	}
	signedAt, err := store.SignedAt("session-a", "checklist-a")
	if err != nil {
		t.Fatal(err)
	}
	if signedAt.IsZero() {
		t.Error("EnsureOpen cleared the existing signature")
	}
}

func TestBeginParticipantAssignsSuffixOnCollision(t *testing.T) {
	store := openTestStore(t)

	first, err := store.BeginParticipant("lemongrass-64", "")
	if err != nil {
		t.Fatalf("BeginParticipant (first): %v", err)
	}
	if first != "lemongrass-64" {
		t.Errorf("first BeginParticipant = %q, want no suffix", first)
	}

	second, err := store.BeginParticipant("lemongrass-64", "")
	if err != nil {
		t.Fatalf("BeginParticipant (second): %v", err)
	}
	if second != "lemongrass-64-2" {
		t.Errorf("second BeginParticipant = %q, want %q", second, "lemongrass-64-2")
	}
}

func TestBeginParticipantReusesNameOnceEnded(t *testing.T) {
	store := openTestStore(t)

	if _, err := store.BeginParticipant("foo", ""); err != nil {
		t.Fatalf("BeginParticipant: %v", err)
	}
	if err := store.EndParticipant("foo"); err != nil {
		t.Fatalf("EndParticipant: %v", err)
	}

	again, err := store.BeginParticipant("foo", "")
	if err != nil {
		t.Fatalf("BeginParticipant (after end): %v", err)
	}
	if again != "foo" {
		t.Errorf("BeginParticipant after EndParticipant = %q, want the name freed up, not suffixed", again)
	}
}

func TestBeginParticipantLinksClaudeSessionID(t *testing.T) {
	store := openTestStore(t)

	if _, err := store.BeginParticipant("foo", "claude-session-a"); err != nil {
		t.Fatalf("BeginParticipant: %v", err)
	}

	p, err := store.ParticipantByName("foo")
	if err != nil {
		t.Fatalf("ParticipantByName: %v", err)
	}
	if p.ClaudeSessionID != "claude-session-a" {
		t.Errorf("ParticipantByName.ClaudeSessionID = %q, want %q", p.ClaudeSessionID, "claude-session-a")
	}
}

func TestParticipantByNameUnknownReturnsZeroValueNoError(t *testing.T) {
	store := openTestStore(t)

	p, err := store.ParticipantByName("nobody")
	if err != nil {
		t.Fatalf("ParticipantByName: %v", err)
	}
	if p.ClaudeSessionID != "" {
		t.Errorf("ParticipantByName(unknown).ClaudeSessionID = %q, want empty", p.ClaudeSessionID)
	}
}

func TestHasOpenSession(t *testing.T) {
	store := openTestStore(t)

	if has, err := store.HasOpenSession("session-a"); err != nil || has {
		t.Fatalf("HasOpenSession before Start = %v, %v, want false, nil", has, err)
	}

	if err := store.Start("session-a", "", ""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if has, err := store.HasOpenSession("session-a"); err != nil || !has {
		t.Fatalf("HasOpenSession after Start = %v, %v, want true, nil", has, err)
	}

	if err := store.End("session-a"); err != nil {
		t.Fatalf("End: %v", err)
	}
	if has, err := store.HasOpenSession("session-a"); err != nil || has {
		t.Fatalf("HasOpenSession after End = %v, %v, want false, nil", has, err)
	}
}

func TestTipsAddListRemove(t *testing.T) {
	store := openTestStore(t)

	id, err := store.AddTip("test tip one")
	if err != nil {
		t.Fatalf("AddTip: %v", err)
	}
	if _, err := store.AddTip("test tip two"); err != nil {
		t.Fatalf("AddTip: %v", err)
	}

	tips, err := store.ListTips()
	if err != nil {
		t.Fatalf("ListTips: %v", err)
	}
	if len(tips) != 2 {
		t.Fatalf("len(tips) = %d, want 2", len(tips))
	}
	if tips[0].ID != id || tips[0].Message != "test tip one" {
		t.Errorf("tips[0] = %+v, want {%d test tip one}", tips[0], id)
	}

	if err := store.DeleteTip(id); err != nil {
		t.Fatalf("DeleteTip: %v", err)
	}
	tips, err = store.ListTips()
	if err != nil {
		t.Fatalf("ListTips after delete: %v", err)
	}
	if len(tips) != 1 || tips[0].Message != "test tip two" {
		t.Fatalf("tips after delete = %+v, want only 'test tip two'", tips)
	}
}

func TestSignedAtZeroBeforeSigning(t *testing.T) {
	store := openTestStore(t)

	signedAt, err := store.SignedAt("session-a", "checklist-1")
	if err != nil {
		t.Fatalf("SignedAt: %v", err)
	}
	if !signedAt.IsZero() {
		t.Errorf("SignedAt before Sign = %v, want zero time", signedAt)
	}

	if err := store.Sign("session-a", "checklist-1"); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	signedAt, err = store.SignedAt("session-a", "checklist-1")
	if err != nil {
		t.Fatalf("SignedAt after Sign: %v", err)
	}
	if signedAt.IsZero() {
		t.Error("SignedAt after Sign is still zero")
	}
	if time.Since(signedAt) > time.Minute {
		t.Errorf("SignedAt = %v, expected close to now", signedAt)
	}
}

func TestSignIsPerSessionAndPerChecklist(t *testing.T) {
	store := openTestStore(t)

	if err := store.Sign("session-a", "checklist-1"); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if signedAt, err := store.SignedAt("session-b", "checklist-1"); err != nil || !signedAt.IsZero() {
		t.Errorf("session-b SignedAt = %v, %v, want zero time, nil (signing doesn't cross sessions)", signedAt, err)
	}
	if signedAt, err := store.SignedAt("session-a", "checklist-2"); err != nil || !signedAt.IsZero() {
		t.Errorf("checklist-2 SignedAt = %v, %v, want zero time, nil (signing doesn't cross checklists)", signedAt, err)
	}
}

func TestReSignResetsTheClock(t *testing.T) {
	store := openTestStore(t)

	if err := store.Sign("session-a", "checklist-1"); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	old := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := store.db.Exec(
		`UPDATE lg_signatures SET signed_at = ? WHERE project_id = ? AND session_id = ? AND checklist_id = ?`,
		old, testProjectID, "session-a", "checklist-1",
	); err != nil {
		t.Fatalf("backdating signature: %v", err)
	}

	if err := store.Sign("session-a", "checklist-1"); err != nil {
		t.Fatalf("re-Sign: %v", err)
	}
	signedAt, err := store.SignedAt("session-a", "checklist-1")
	if err != nil {
		t.Fatalf("SignedAt: %v", err)
	}
	if time.Since(signedAt) > time.Minute {
		t.Errorf("SignedAt after re-sign = %v, expected close to now, not the backdated value", signedAt)
	}
}

func TestStartClearsSignaturesForReusedSessionID(t *testing.T) {
	store := openTestStore(t)

	if err := store.Sign("session-a", "checklist-1"); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if err := store.Start("session-a", "", ""); err != nil {
		t.Fatalf("Start: %v", err)
	}

	signedAt, err := store.SignedAt("session-a", "checklist-1")
	if err != nil {
		t.Fatalf("SignedAt: %v", err)
	}
	if !signedAt.IsZero() {
		t.Errorf("SignedAt after Start = %v, want zero time (a reused session_id must not inherit a stale sign)", signedAt)
	}
}

func TestRecordTabSessionReplacesTheTabsSession(t *testing.T) {
	store := openTestStore(t)

	if err := store.RecordTabSession("tab-1", "session-a"); err != nil {
		t.Fatalf("RecordTabSession a: %v", err)
	}
	if err := store.RecordTabSession("tab-1", "session-b"); err != nil {
		t.Fatalf("RecordTabSession b: %v", err)
	}
	if err := store.RecordTabSession("tab-2", "session-c"); err != nil {
		t.Fatalf("RecordTabSession c: %v", err)
	}

	tabs, err := store.TabSessions()
	if err != nil {
		t.Fatalf("TabSessions: %v", err)
	}
	if len(tabs) != 2 || tabs["tab-1"] != "session-b" || tabs["tab-2"] != "session-c" {
		t.Errorf("TabSessions = %v, want tab-1=session-b and tab-2=session-c", tabs)
	}
}

func TestTabSessionsAreScopedToTheProject(t *testing.T) {
	store := openTestStore(t)
	other, err := Open(DBPath(), "otherproj")
	if err != nil {
		t.Fatalf("Open other: %v", err)
	}
	t.Cleanup(func() { other.Close() })

	if err := store.RecordTabSession("tab-1", "session-a"); err != nil {
		t.Fatalf("RecordTabSession: %v", err)
	}
	tabs, err := other.TabSessions()
	if err != nil {
		t.Fatalf("TabSessions: %v", err)
	}
	if len(tabs) != 0 {
		t.Errorf("other project sees %v, want none", tabs)
	}
}

func TestForgetTabSessionRemovesOnlyThatTab(t *testing.T) {
	store := openTestStore(t)
	for _, tab := range []string{"tab-1", "tab-2"} {
		if err := store.RecordTabSession(tab, "session-"+tab); err != nil {
			t.Fatalf("RecordTabSession %s: %v", tab, err)
		}
	}

	if err := store.ForgetTabSession("tab-1"); err != nil {
		t.Fatalf("ForgetTabSession: %v", err)
	}
	tabs, _ := store.TabSessions()
	if len(tabs) != 1 || tabs["tab-2"] != "session-tab-2" {
		t.Errorf("after forgetting tab-1, TabSessions = %v", tabs)
	}
}
