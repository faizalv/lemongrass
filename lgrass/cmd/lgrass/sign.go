package main

import (
	"fmt"
	"os"

	"github.com/faizalv/lemongrass/session"
)

// The model-facing side of the prerequisite gate: satisfies a checklist's PreToolUse deny for this session, for that checklist's own TTL.
func cmdSign(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrass sign <checklist-id>")
		os.Exit(1)
	}
	checklistID := args[0]

	sessionID := os.Getenv("CLAUDE_CODE_SESSION_ID")
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "error: CLAUDE_CODE_SESSION_ID is not set -- lgrass sign only works invoked from within a session that has one")
		os.Exit(1)
	}

	proj := currentProject()
	store, err := session.Open(session.DBPath(), proj.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.Sign(sessionID, checklistID); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("signed %q.\n", checklistID)
}
