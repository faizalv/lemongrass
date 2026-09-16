package main

import (
	"encoding/json"
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
			payload := hookPayload{ToolName: c.toolName, ToolInput: json.RawMessage(c.input)}
			if got := isBibliothekSkillCall(payload); got != c.want {
				t.Errorf("isBibliothekSkillCall(%q, %q) = %v, want %v", c.toolName, c.input, got, c.want)
			}
		})
	}
}

func TestBibliothekDenyUntilSigned(t *testing.T) {
	store := openHookTestStore(t)

	if deny := bibliothekDeny(store, "session-a"); deny == "" {
		t.Error("bibliothekDeny before signing = \"\", want a deny message")
	}

	if err := store.Sign("session-a", bibliothekChecklistID); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if deny := bibliothekDeny(store, "session-a"); deny != "" {
		t.Errorf("bibliothekDeny after signing = %q, want \"\"", deny)
	}
}

func TestBibliothekDenyIsPerSession(t *testing.T) {
	store := openHookTestStore(t)

	if err := store.Sign("session-a", bibliothekChecklistID); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if deny := bibliothekDeny(store, "session-b"); deny == "" {
		t.Error("bibliothekDeny for a different, unsigned session = \"\", want a deny message (signing doesn't cross sessions)")
	}
}
