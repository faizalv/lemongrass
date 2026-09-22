package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/faizalv/lemongrass/session"
)

// The model-facing side of the prerequisite gate: satisfies a checklist's PreToolUse deny for this session, for that checklist's own TTL.
func cmdSign(args []string) {
	checklistID, sessionID, err := signRequest(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "usage: lgrass sign [--session-id <id>] <checklist-id>")
		os.Exit(1)
	}
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "error: no session id is available. Pass --session-id when the agent does not export CLAUDE_CODE_SESSION_ID")
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

func signRequest(args []string) (checklistID, sessionID string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--session-id":
			i++
			if i >= len(args) || args[i] == "" {
				return "", "", errors.New("missing session id")
			}
			sessionID = args[i]
		default:
			if checklistID != "" {
				return "", "", errors.New("multiple checklist ids")
			}
			checklistID = args[i]
		}
	}
	if checklistID == "" {
		return "", "", errors.New("missing checklist id")
	}
	if sessionID == "" {
		sessionID = os.Getenv("CLAUDE_CODE_SESSION_ID")
	}
	return checklistID, sessionID, nil
}
