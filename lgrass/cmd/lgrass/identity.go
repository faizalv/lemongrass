package main

import (
	"fmt"
	"os"

	"github.com/faizalv/lemongrass/session"
)

// currentParticipantName returns LGRASS_SESSION if set, else CLAUDE_CODE_SESSION_ID, erroring if neither is.
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

// currentClaudeSessionExclusion resolves this invocation's own Claude session_id, via LGRASS_SESSION's linked participant when set.
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
