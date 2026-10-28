// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"strings"

	"github.com/faizalv/lemongrass/internal/session"
	"github.com/faizalv/lemongrass/internal/threadsvc"
)

// Records which of the agent's own hook events fired last for the tab, so a typed nudge never lands on an open permission prompt or question picker.
func recordTabState(store *session.Store, event string, payload hookEvent) {
	tab := payload.TabID
	if tab == "" {
		return
	}
	switch event {
	case "SessionStart":
		store.SetTabState(tab, session.StateIdle)
	case "SessionEnd":
		store.ClearTabState(tab)
	case "PreToolUse":
		// The question picker takes an Enter as a choice, so it blocks typing like a permission prompt until its PostToolUse.
		if payload.ToolName == "AskUserQuestion" {
			store.SetTabState(tab, session.StatePrompting)
		} else {
			store.SetTabState(tab, session.StateWorking)
		}
	case "PostToolUse", "UserPromptSubmit":
		store.SetTabState(tab, session.StateWorking)
	case "PermissionRequest":
		store.SetTabState(tab, session.StatePrompting)
	case "Notification":
		if isPermissionPrompt(payload) {
			store.SetTabState(tab, session.StatePrompting)
		} else if payload.NotificationType == "idle_prompt" {
			store.SetTabState(tab, session.StateIdle)
		}
	case "Stop", "Interrupt":
		store.SetTabState(tab, session.StateIdle)
		if pending, err := store.PendingForTab(tab); err == nil && !pending.Empty() {
			threadsvc.Wake(threadsvc.SocketPath())
		}
	}
}

// Leans towards a prompt when the type is missing, since a nudge typed into one could answer it.
func isPermissionPrompt(payload hookEvent) bool {
	if payload.NotificationType != "" {
		return payload.NotificationType == "permission_prompt"
	}
	return strings.Contains(strings.ToLower(payload.Message), "permission")
}
