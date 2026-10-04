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
		fmt.Fprintln(os.Stderr, "usage: lgrass session <list|tabs> ...")
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		cmdSessionList(args[1:])
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

	exclude := currentSessionExclusion(store)
	liveness, err := store.Liveness(exclude, idleThresholdForList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(liveness) == 0 {
		fmt.Println("lgrass: no other sessions open in this project.")
		return
	}
	tabBySession := map[string]string{}
	if tabs, err := store.TabSessions(); err == nil {
		for tab, sessionID := range tabs {
			tabBySession[sessionID] = tab
		}
	}
	vendors, _ := store.TabVendors()
	for _, s := range liveness {
		status := "idling"
		if s.Active {
			status = "active"
		}
		line := fmt.Sprintf("%s  %s", s.SessionID, status)
		if tab := tabBySession[s.SessionID]; tab != "" {
			line += fmt.Sprintf("  tab %s", tab)
			if vendor := vendors[tab]; vendor != "" {
				line += "  " + vendor
			}
		}
		fmt.Println(line)
	}
}
