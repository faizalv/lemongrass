package main

import (
	"fmt"
	"os"

	"github.com/faizalv/lemongrass/session"
)

// Falls back to CLAUDE_CODE_SESSION_ID so a session that hasn't called `session begin` yet still works.
func currentParticipantName() string {
	if name := os.Getenv("LGRASS_SESSION"); name != "" {
		return name
	}
	id := os.Getenv("CLAUDE_CODE_SESSION_ID")
	if id == "" {
		fmt.Fprintln(os.Stderr, "error: neither LGRASS_SESSION nor CLAUDE_CODE_SESSION_ID is set -- lgrass thread only works invoked from within a session that has one")
		os.Exit(1)
	}
	return id
}

func currentClaudeSessionExclusion(store *session.Store) string {
	name := os.Getenv("LGRASS_SESSION")
	if name == "" {
		return os.Getenv("CLAUDE_CODE_SESSION_ID")
	}
	p, err := store.ParticipantByName(name)
	if err != nil {
		return ""
	}
	return p.ClaudeSessionID
}
