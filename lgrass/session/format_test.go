package session

import (
	"strings"
	"testing"
)

func TestFormatCollisionWarningEmpty(t *testing.T) {
	if got := FormatCollisionWarning(nil); got != "" {
		t.Errorf("FormatCollisionWarning(nil) = %q, want empty string", got)
	}
}

func TestFormatCollisionWarningDistinguishesSameFileAndFolder(t *testing.T) {
	hits := []ActivityHit{
		{SessionID: "session-b", FilePath: "/proj/pkg/foo.go", SameFile: true},
		{SessionID: "session-c", FilePath: "/proj/pkg/bar.go", SameFile: false},
	}
	got := FormatCollisionWarning(hits)
	if !strings.Contains(got, "session-b touched /proj/pkg/foo.go (same file)") {
		t.Errorf("missing same-file line, got:\n%s", got)
	}
	if !strings.Contains(got, "session-c touched /proj/pkg/bar.go (same folder)") {
		t.Errorf("missing same-folder line, got:\n%s", got)
	}
}

func TestFormatNudgeNoOtherSessions(t *testing.T) {
	got := FormatNudge(nil)
	if !strings.Contains(got, "0 other session(s)") {
		t.Errorf("FormatNudge(nil) = %q, want it to report 0 other sessions", got)
	}
	if strings.Contains(got, "active") {
		t.Errorf("FormatNudge(nil) = %q, should not break down active/idling with no other sessions", got)
	}
}

func TestFormatNudgeCountsActiveAndIdling(t *testing.T) {
	liveness := []SessionStatus{
		{SessionID: "a", Active: true},
		{SessionID: "b", Active: true},
		{SessionID: "c", Active: false},
	}
	got := FormatNudge(liveness)
	if !strings.Contains(got, "3 other session(s) open in this project (2 active, 1 idling)") {
		t.Errorf("FormatNudge = %q, want a 3/2/1 breakdown", got)
	}
}

func TestFormatThreadPushReadsAsNotTheHuman(t *testing.T) {
	got := FormatThreadPush("session-a", "schema migration finished")
	if !strings.Contains(got, "not your user") {
		t.Errorf("FormatThreadPush = %q, missing the not-the-human framing", got)
	}
	if !strings.Contains(got, "session session-a: schema migration finished") {
		t.Errorf("FormatThreadPush = %q, missing sender attribution and body", got)
	}
}

func TestFormatMentionsEmpty(t *testing.T) {
	if got := FormatMentions(nil); got != "" {
		t.Errorf("FormatMentions(nil) = %q, want empty string", got)
	}
}

func TestFormatMentionsListsEachMessage(t *testing.T) {
	msgs := []ThreadMessage{
		{SessionID: "session-a", Body: "here's the shape I need"},
		{SessionID: "session-a", Body: "second message"},
	}
	got := FormatMentions(msgs)
	if !strings.Contains(got, "you were mentioned") {
		t.Errorf("FormatMentions = %q, missing mention framing", got)
	}
	if !strings.Contains(got, "session-a: here's the shape I need") || !strings.Contains(got, "session-a: second message") {
		t.Errorf("FormatMentions = %q, missing one or both messages", got)
	}
}

func TestFormatMemoryFeedbackDeny(t *testing.T) {
	got := FormatMemoryFeedbackDeny()
	if !strings.Contains(got, "lgrass sign memory-feedback-law") {
		t.Errorf("FormatMemoryFeedbackDeny() = %q, missing the sign command", got)
	}
	if !strings.Contains(got, "biblio/laws") {
		t.Errorf("FormatMemoryFeedbackDeny() = %q, missing the biblio/laws pointer", got)
	}
}

func TestFormatThreadListEmpty(t *testing.T) {
	got := FormatThreadList(nil)
	if !strings.Contains(got, "no thread messages yet") {
		t.Errorf("FormatThreadList(nil) = %q, want an empty-state message", got)
	}
}

func TestFormatThreadListShowsMentionTarget(t *testing.T) {
	msgs := []ThreadMessage{
		{SessionID: "session-a", Mention: "session-b", Body: "check this", CreatedAt: "2026-09-04T00:00:00Z"},
		{SessionID: "session-c", Body: "no target", CreatedAt: "2026-09-04T00:01:00Z"},
	}
	got := FormatThreadList(msgs)
	if !strings.Contains(got, "session-a -> session-b: check this") {
		t.Errorf("FormatThreadList = %q, missing mention arrow line", got)
	}
	if !strings.Contains(got, "session-c: no target") {
		t.Errorf("FormatThreadList = %q, missing unmentioned line", got)
	}
	if strings.Contains(got, "session-c ->") {
		t.Errorf("FormatThreadList = %q, unmentioned message should not show a -> target", got)
	}
}
