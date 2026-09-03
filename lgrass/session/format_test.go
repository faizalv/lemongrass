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
