// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"encoding/json"
	"os"
)

type claudeHookAdapter struct{}

// A prompt block is a top-level decision in Claude Code's hook schema, not part of hookSpecificOutput.
type claudeHookOutput struct {
	Decision           string                    `json:"decision,omitempty"`
	Reason             string                    `json:"reason,omitempty"`
	HookSpecificOutput *claudeHookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

type claudeHookSpecificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	AdditionalContext        string `json:"additionalContext,omitempty"`
}

func (claudeHookAdapter) decode(raw []byte) (hookEvent, error) {
	var event hookEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return hookEvent{}, err
	}
	event.EnforceBibliothek = true
	return event, nil
}

func (claudeHookAdapter) emit(result hookResult) {
	if !result.hasOutput() {
		return
	}
	json.NewEncoder(os.Stdout).Encode(claudeOutput(result))
}

func claudeOutput(result hookResult) claudeHookOutput {
	if result.BlockReason != "" {
		return claudeHookOutput{Decision: "block", Reason: result.BlockReason}
	}
	context := result.AdditionalContext
	if result.PermissionDecision == "deny" {
		context = ""
	}
	return claudeHookOutput{HookSpecificOutput: &claudeHookSpecificOutput{
		HookEventName:            result.EventName,
		PermissionDecision:       result.PermissionDecision,
		PermissionDecisionReason: result.PermissionDecisionReason,
		AdditionalContext:        context,
	}}
}
