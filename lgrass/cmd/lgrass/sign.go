package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/faizalv/lemongrass/session"
)

const signUsage = `usage: lgrass sign [--session-id <id>] <word>

Signs a word for the current session. A deny message names the word to sign, and
.lgrass/checklists.json can define more. A sign prints the word's pledge, the rule it
stands for, and satisfies the deny until the word's TTL expires. A word that is not
registered is rejected.

  --session-id <id>   the session to sign for, needed when the agent does not export
                      CLAUDE_CODE_SESSION_ID
`

// The model-facing side of the prerequisite gate: satisfies a registered word's PreToolUse deny for this session, for that word's own TTL, and answers with its pledge.
func cmdSign(args []string) {
	if signHelpRequested(args) {
		fmt.Print(signUsage)
		return
	}
	checklistID, sessionID, err := signRequest(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n\n%s", err, signUsage)
		os.Exit(1)
	}
	proj := currentProject()
	signable, ok, err := session.ResolveSignable(proj.Path, checklistID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unknown checklist id %q\n", checklistID)
		os.Exit(1)
	}
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "error: no session id is available. Pass --session-id when the agent does not export CLAUDE_CODE_SESSION_ID")
		os.Exit(1)
	}

	store, err := session.Open(session.DBPath(), proj.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.Sign(sessionID, signable.ID); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(signAcknowledgement(signable))
}

func signHelpRequested(args []string) bool {
	if len(args) == 0 {
		return true
	}
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

func signAcknowledgement(s session.Signable) string {
	if s.Pledge == "" {
		return "signed " + s.ID + "."
	}
	return "signed " + s.ID + ". " + s.Pledge
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
