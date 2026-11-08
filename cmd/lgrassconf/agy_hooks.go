// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import "github.com/faizalv/lemongrass/internal/agentconf"

func reconcileAgyHooks(data []byte, lgrassdPath string) ([]byte, bool, error) {
	events := map[string]any{}
	for _, event := range []string{"SessionStart", "PreToolUse", "PostToolUse", "PreInvocation", "Stop"} {
		mapped := event
		if event == "PreInvocation" {
			mapped = "UserPromptSubmit"
		}
		handler := agentconf.HookHandler{Type: "command", Command: agentconf.HookCommand(lgrassdPath, "hook", mapped, agentconf.VendorFlag, "agy")}
		if event == "PreToolUse" || event == "PostToolUse" {
			events[event] = []agentconf.HookGroup{{Matcher: "*", Hooks: []agentconf.HookHandler{handler}}}
		} else {
			events[event] = []agentconf.HookHandler{handler}
		}
	}
	return agentconf.ReconcileNamedHooks(data, "lemongrass", events)
}
