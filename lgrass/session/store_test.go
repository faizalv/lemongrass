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

	if err := store.Start("session-a"); err != nil {
		t.Fatalf("Start a: %v", err)
	}
	if err := store.Start("session-b"); err != nil {
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

	if err := store.Start("session-a"); err != nil {
		t.Fatalf("Start a: %v", err)
	}
	if err := store.Start("session-b"); err != nil {
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

	if err := store.Start("session-a"); err != nil {
		t.Fatalf("Start a: %v", err)
	}
	if err := store.Start("session-b"); err != nil {
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

	if err := store.Start("session-a"); err != nil {
		t.Fatalf("Start a: %v", err)
	}
	if err := store.Start("session-b"); err != nil {
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

	// Fresh activity, but the session has since ended -- no longer "live".
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
	if err := store.Start("session-a"); err != nil {
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

	// Counter should have reset -- another threshold-1 calls shouldn't fire again.
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

func TestIncrementNudgeCounterWithoutPriorStart(t *testing.T) {
	store := openTestStore(t)
	// No Start call -- defensive upsert path should still work.
	fire, err := store.IncrementNudgeCounter("session-a", 1)
	if err != nil {
		t.Fatalf("IncrementNudgeCounter: %v", err)
	}
	if !fire {
		t.Error("threshold of 1 should fire on the first call even without a prior Start")
	}
}
