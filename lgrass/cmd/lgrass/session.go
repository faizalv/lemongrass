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
	if len(args) == 0 || args[0] != "list" {
		fmt.Fprintln(os.Stderr, "usage: lgrass session list")
		os.Exit(1)
	}

	sessionID := currentSessionID()
	proj := currentProject()
	store, err := session.Open(session.DBPath(), proj.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	liveness, err := store.Liveness(sessionID, idleThresholdForList)
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
