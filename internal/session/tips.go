// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package session

import "math/rand"

// Short reminders surfaced periodically alongside the regular nudge, only in a project with biblio/. They bring rules
// back into relevance late in a long session, so each stays a few words and must match the current biblio convention
// and lgrass how content.
var tips = []string{
	"tip: finished work gets closed, never handed over.",
	"tip: scratchpad and handover hold only work in flight.",
	"tip: chapters record how it works, never how it broke.",
	"tip: memory holds pointers only, never facts.",
	"tip: every task file sits in its task directory.",
	"tip: unread part? Open its book first.",
	"tip: handover Status is current, its Log is history.",
	"tip: task close archives the task and its handover.",
	"tip: one camelCase tag per compound idea.",
	"tip: unstarted work gets a PRD, not a handover.",
	"tip: you write in scratchpad too.",
	"tip: only the user ticks User approval.",
	"tip: PRDs use only the standard headings.",
	"tip: `lgrass how` has the manual for each action.",
}

func RandomTip() string {
	return RandomTipFrom(nil)
}

// RandomTipFrom picks from the built-in tips plus whatever user-authored ones this project has configured.
func RandomTipFrom(extra []string) string {
	all := append(append([]string{}, tips...), extra...)
	return all[rand.Intn(len(all))]
}
