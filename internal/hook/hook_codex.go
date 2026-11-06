// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"encoding/json"
	"os"
)

type codexHookAdapter struct{}

// Codex follows Claude Code's hook schema, so a prompt block is a top-level decision here too.
type codexHookOutput struct {
	Decision           string                   `json:"decision,omitempty"`
	Reason             string                   `json:"reason,omitempty"`
	HookSpecificOutput *codexHookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

type codexHookSpecificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	AdditionalContext        string `json:"additionalContext,omitempty"`
}

func (codexHookAdapter) decode(raw []byte) (hookEvent, error) {
	var event hookEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return hookEvent{}, err
	}
	event.PreserveSessionState = true
	return event, nil
}

func (codexHookAdapter) emit(result hookResult) {
	if !result.hasOutput() {
		return
	}
	json.NewEncoder(os.Stdout).Encode(codexOutput(result))
}

func codexOutput(result hookResult) codexHookOutput {
	if result.BlockReason != "" {
		return codexHookOutput{Decision: "block", Reason: result.BlockReason}
	}
	return codexHookOutput{HookSpecificOutput: &codexHookSpecificOutput{
		HookEventName:            result.EventName,
		PermissionDecision:       result.PermissionDecision,
		PermissionDecisionReason: result.PermissionDecisionReason,
		AdditionalContext:        result.AdditionalContext,
	}}
}
