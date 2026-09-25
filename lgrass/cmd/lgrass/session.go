package main

import (
	"fmt"
	"os"
	"time"

	"github.com/faizalv/lemongrass/session"
)

// idleThresholdForList matches hook.go's idleThreshold, the same
// active/idling definition either way.
const idleThresholdForList = 5 * time.Minute

func cmdSession(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrass session <list|begin|end|tabs> ...")
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		cmdSessionList(args[1:])
	case "begin":
		cmdSessionBegin(args[1:])
	case "end":
		cmdSessionEnd(args[1:])
	case "tabs":
		cmdSessionTabs(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown session command: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdSessionList(args []string) {
	proj := currentProject()
	store, err := session.Open(session.DBPath(), proj.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	exclude := currentClaudeSessionExclusion(store)
	liveness, err := store.Liveness(exclude, idleThresholdForList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(liveness) == 0 {
		fmt.Println("lgrass: no other sessions open in this project.")
		return
	}
	for _, s := range liveness {
		status := "idling"
		if s.Active {
			status = "active"
		}
		fmt.Printf("%s  %s\n", s.SessionID, status)
	}
}

// cmdSessionBegin prints the assigned participant name bare to stdout, for a caller to capture directly.
func cmdSessionBegin(args []string) {
	var name string
	for i := 0; i < len(args); i++ {
		if args[i] == "--name" {
			i++
			if i < len(args) {
				name = args[i]
			}
		}
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "usage: lgrass session begin --name <name>")
		os.Exit(1)
	}

	proj := currentProject()
	store, err := session.Open(session.DBPath(), proj.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	claudeSessionID := os.Getenv("CLAUDE_CODE_SESSION_ID")
	if claudeSessionID != "" {
		has, err := store.HasOpenSession(claudeSessionID)
		if err != nil || !has {
			claudeSessionID = ""
		}
	}

	assigned, err := store.BeginParticipant(name, claudeSessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if assigned != name {
		fmt.Fprintf(os.Stderr, "lgrass: %q already in use, assigned %q\n", name, assigned)
	}
	fmt.Println(assigned)
}

func cmdSessionEnd(args []string) {
	name := os.Getenv("LGRASS_SESSION")
	if name == "" {
		fmt.Fprintln(os.Stderr, "usage: lgrass session end (requires LGRASS_SESSION to be set)")
		os.Exit(1)
	}

	proj := currentProject()
	store, err := session.Open(session.DBPath(), proj.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.EndParticipant(name); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("lgrass: participant ended.")
}
