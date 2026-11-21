// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/faizalv/lemongrass/internal/agentusage"
	"github.com/faizalv/lemongrass/internal/claudeaccount"
	"github.com/faizalv/lemongrass/internal/session"
)

// cmdUsage prints the last known quota of every supported agent as JSON, for the app.
func cmdUsage(args []string) {
	if len(args) >= 1 && args[0] == "context" && len(args) <= 2 && (len(args) == 1 || args[1] == "--backfill") {
		cmdUsageContext(len(args) == 2)
		return
	}
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrassd usage [context [--backfill]]")
		os.Exit(1)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fail(err)
	}
	names, err := claudeaccount.Accounts{Home: home}.Names()
	if err != nil {
		fail(err)
	}
	accounts := append([]string{"primary"}, names...)
	report := agentusage.Collect(home, agentusage.CodexHome(home), accounts, time.Now())
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fail(err)
	}
}

// cmdUsageContext prints each agent tab's context window use by tab id, and the directory those records live in so
// the app can watch it without knowing the layout. The directory is created here, since a watch needs it to exist.
// --backfill also fills cache lifetimes missing from older records; the app asks for it once per start, since it
// searches transcripts.
func cmdUsageContext(backfill bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		fail(err)
	}
	dir := agentusage.ContextDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(err)
	}
	tabs, err := agentusage.ReadContexts(home)
	if err != nil {
		fail(err)
	}
	if backfill {
		if store, err := session.Open(session.DBPath(), ""); err == nil {
			if sessions, err := store.TabSessionsAllProjects(); err == nil {
				agentusage.BackfillCacheTTL(home, tabs, sessions)
			}
			store.Close()
		}
	}
	out := struct {
		Dir  string                        `json:"dir"`
		Tabs map[string]agentusage.Context `json:"tabs"`
	}{dir, tabs}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fail(err)
	}
}
