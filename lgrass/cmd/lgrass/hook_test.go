package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/session"
)

const hookTestProjectID = "hooktestproj"

func openHookTestStore(t *testing.T) *session.Store {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	store, err := session.Open(session.DBPath(), hookTestProjectID)
	if err != nil {
		t.Fatalf("session.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestIsBibliothekSkillCall(t *testing.T) {
	cases := []struct {
		name     string
		toolName string
		input    string
		want     bool
	}{
		{"bibliothek skill call", "Skill", `{"skill":"bibliothek"}`, true},
		{"other skill call", "Skill", `{"skill":"code-review"}`, false},
		{"non-skill tool", "Bash", `{"command":"ls"}`, false},
		{"malformed input", "Skill", `not json`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			payload := hookEvent{ToolName: c.toolName, ToolInput: json.RawMessage(c.input)}
			if got := isBibliothekSkillCall(payload); got != c.want {
				t.Errorf("isBibliothekSkillCall(%q, %q) = %v, want %v", c.toolName, c.input, got, c.want)
			}
		})
	}
}

func TestPatchFilePaths(t *testing.T) {
	command := "*** Begin Patch\n*** Update File: one.txt\n*** Add File: two.txt\n*** Update File: one.txt\n*** Delete File: three.txt\n*** End Patch"
	got := patchFilePaths(command)
	want := []string{"one.txt", "two.txt", "three.txt"}
	if len(got) != len(want) {
		t.Fatalf("patchFilePaths() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("patchFilePaths()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCodexHookAdapterPreservesSessionState(t *testing.T) {
	adapter := codexHookAdapter{}
	event, err := adapter.decode([]byte(`{"session_id":"session-a","cwd":"/workspace","source":"compact"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !event.PreserveSessionState {
		t.Error("Codex event should preserve existing session state")
	}
	if event.SessionSource != "compact" {
		t.Errorf("SessionSource = %q, want compact", event.SessionSource)
	}
}

func TestCodexChecklistDenyUsesExplicitSessionID(t *testing.T) {
	store := openHookTestStore(t)
	projectPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectPath, ".lgrass"), 0o755); err != nil {
		t.Fatal(err)
	}
	checklist := `[{"id":"review","tool":"Bash","content":"Review the checklist."}]`
	if err := os.WriteFile(filepath.Join(projectPath, ".lgrass", "checklists.json"), []byte(checklist), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := hookEvent{SessionID: "codex-session", ToolName: "Bash", PreserveSessionState: true}
	deny := checklistDeny(store, payload, nil, projectPath)
	if !strings.Contains(deny, "lgrass sign --session-id codex-session review") {
		t.Errorf("checklistDeny() = %q, want a Codex session signing command", deny)
	}
}

func TestBibliothekDenyUntilSigned(t *testing.T) {
	store := openHookTestStore(t)

	if deny := bibliothekDeny(store, "session-a", ""); deny == "" {
		t.Error("bibliothekDeny before signing = \"\", want a deny message")
	}

	if err := store.Sign("session-a", bibliothekChecklistID); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if deny := bibliothekDeny(store, "session-a", ""); deny != "" {
		t.Errorf("bibliothekDeny after signing = %q, want \"\"", deny)
	}
}

func TestBibliothekDenyIsPerSession(t *testing.T) {
	store := openHookTestStore(t)

	if err := store.Sign("session-a", bibliothekChecklistID); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if deny := bibliothekDeny(store, "session-b", ""); deny == "" {
		t.Error("bibliothekDeny for a different, unsigned session = \"\", want a deny message (signing doesn't cross sessions)")
	}
}

func writeTranscriptLines(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestTranscriptHasBibliothekInvocation(t *testing.T) {
	bibliothekCall := `{"message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"bibliothek"}}]}}`
	otherSkillCall := `{"message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"code-review"}}]}}`
	textTurn := `{"message":{"content":"just a plain user message"}}`

	cases := []struct {
		name  string
		lines []string
		want  bool
	}{
		{"finds the call among unrelated lines", []string{textTurn, otherSkillCall, bibliothekCall}, true},
		{"absent", []string{textTurn, otherSkillCall}, false},
		{"malformed line tolerated", []string{"not json", bibliothekCall}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writeTranscriptLines(t, c.lines...)
			if got := transcriptHasBibliothekInvocation(path); got != c.want {
				t.Errorf("transcriptHasBibliothekInvocation() = %v, want %v", got, c.want)
			}
		})
	}

	if transcriptHasBibliothekInvocation("") {
		t.Error("transcriptHasBibliothekInvocation(\"\") = true, want false")
	}
	if transcriptHasBibliothekInvocation(filepath.Join(t.TempDir(), "missing.jsonl")) {
		t.Error("transcriptHasBibliothekInvocation(missing file) = true, want false")
	}
}

// Covers the harness-dedup dead end: a session never got its own PreToolUse
// hook cycle for the Skill(bibliothek) call, but the transcript still proves
// it happened, so the gate must self-heal instead of denying forever.
func TestBibliothekDenyRecoversFromTranscript(t *testing.T) {
	store := openHookTestStore(t)
	path := writeTranscriptLines(t, `{"message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"bibliothek"}}]}}`)

	if deny := bibliothekDeny(store, "session-a", path); deny != "" {
		t.Errorf("bibliothekDeny with a transcript proving invocation = %q, want \"\"", deny)
	}

	signedAt, err := store.SignedAt("session-a", bibliothekChecklistID)
	if err != nil {
		t.Fatalf("SignedAt: %v", err)
	}
	if signedAt.IsZero() {
		t.Error("bibliothekDeny found the transcript marker but did not sign the session")
	}
}
