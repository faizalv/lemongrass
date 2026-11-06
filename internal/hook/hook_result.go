// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import "strings"

type hookResult struct {
	EventName                string
	PermissionDecision       string
	PermissionDecisionReason string
	AdditionalContext        string
	// BlockReason, when set, rejects a UserPromptSubmit and is shown to the human in place of the prompt.
	BlockReason string
}

func newHookResult(eventName, permissionDecision string, parts []string) hookResult {
	var nonEmpty []string
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	context := strings.Join(nonEmpty, "\n\n")
	result := hookResult{
		EventName:          eventName,
		PermissionDecision: permissionDecision,
		AdditionalContext:  context,
	}
	if permissionDecision == "deny" {
		result.PermissionDecisionReason = context
	}
	return result
}

func (r hookResult) hasOutput() bool {
	return r.PermissionDecision == "deny" || r.AdditionalContext != "" || r.BlockReason != ""
}
