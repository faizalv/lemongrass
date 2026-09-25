package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/faizalv/lemongrass/session"
)

func cmdSessionTabs(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrass session tabs <list|forget> ...")
		os.Exit(1)
	}

	proj := currentProject()
	store, err := session.Open(session.DBPath(), proj.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	switch args[0] {
	case "list":
		tabs, err := store.TabSessions()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		json.NewEncoder(os.Stdout).Encode(tabs)
	case "forget":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: lgrass session tabs forget <tab-id>")
			os.Exit(1)
		}
		if err := store.ForgetTabSession(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown session tabs command: %s\n", args[0])
		os.Exit(1)
	}
}
