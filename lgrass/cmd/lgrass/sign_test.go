package main

import "testing"

func TestSignRequest(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "claude-session")
	cases := []struct {
		name        string
		args        []string
		wantID      string
		wantSession string
		wantErr     bool
	}{
		{"Claude environment", []string{"review"}, "review", "claude-session", false},
		{"explicit Codex session", []string{"--session-id", "codex-session", "review"}, "review", "codex-session", false},
		{"missing checklist", nil, "", "", true},
		{"missing session flag value", []string{"--session-id"}, "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, sessionID, err := signRequest(c.args)
			if (err != nil) != c.wantErr {
				t.Fatalf("signRequest(%v) error = %v, wantErr %v", c.args, err, c.wantErr)
			}
			if id != c.wantID || sessionID != c.wantSession {
				t.Errorf("signRequest(%v) = (%q, %q), want (%q, %q)", c.args, id, sessionID, c.wantID, c.wantSession)
			}
		})
	}
}
