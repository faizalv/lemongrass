package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/faizalv/lemongrass/project"
	"github.com/faizalv/lemongrass/session"
)

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func openStore() *session.Store {
	proj, err := project.ResolveCwd()
	if err != nil {
		fail(err)
	}
	store, err := session.Open(session.DBPath(), proj.ID)
	if err != nil {
		fail(err)
	}
	return store
}

func cmdTabs(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrassd tabs <list|register|title|forget> ...")
		os.Exit(1)
	}
	store := openStore()
	defer store.Close()

	switch args[0] {
	case "list":
		tabs, err := store.TabSessions()
		if err != nil {
			fail(err)
		}
		json.NewEncoder(os.Stdout).Encode(tabs)
	case "register":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: lgrassd tabs register <tab-id> <vendor>")
			os.Exit(1)
		}
		if err := store.RegisterTab(args[1], args[2]); err != nil {
			fail(err)
		}
	case "title":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: lgrassd tabs title <tab-id> <title>")
			os.Exit(1)
		}
		if err := store.SetTabTitle(args[1], args[2]); err != nil {
			fail(err)
		}
	case "forget":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: lgrassd tabs forget <tab-id>")
			os.Exit(1)
		}
		if err := store.ForgetTab(args[1]); err != nil {
			fail(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown tabs command: %s\n", args[0])
		os.Exit(1)
	}
}
