package main

import (
	"os"

	"github.com/faizalv/lemongrass/session"
)

// Set by the Electron app on each agent tab it spawns, so the id is inherited by everything the tab runs.
const tabIDEnv = "LGRASS_TAB_ID"

// Read once in main(); empty for commands run outside a lemongrass tab. A model can override it on its own command line, so it routes and labels but never authorizes.
var tabID string

func currentSessionExclusion(store *session.Store) string {
	if tabID != "" {
		if id, err := store.SessionForTab(tabID); err == nil && id != "" {
			return id
		}
	}
	return os.Getenv("CLAUDE_CODE_SESSION_ID")
}
