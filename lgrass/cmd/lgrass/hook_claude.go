package main

import (
	"encoding/json"
	"os"
)

type claudeHookAdapter struct{}

type claudeHookOutput struct {
	HookSpecificOutput claudeHookSpecificOutput `json:"hookSpecificOutput"`
}

type claudeHookSpecificOutput struct {
	HookEventName      string `json:"hookEventName"`
	PermissionDecision string `json:"permissionDecision,omitempty"`
	AdditionalContext  string `json:"additionalContext,omitempty"`
}

func (claudeHookAdapter) decode(raw []byte) (hookEvent, error) {
	var event hookEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return hookEvent{}, err
	}
	event.MessagingSocket = os.Getenv("CLAUDE_CODE_MESSAGING_SOCKET")
	event.MessagingToken = os.Getenv("CLAUDE_CODE_MESSAGING_TOKEN")
	event.EnforceBibliothek = true
	return event, nil
}

func (claudeHookAdapter) emit(result hookResult) {
	if !result.hasOutput() {
		return
	}
	json.NewEncoder(os.Stdout).Encode(claudeHookOutput{HookSpecificOutput: claudeHookSpecificOutput{
		HookEventName:      result.EventName,
		PermissionDecision: result.PermissionDecision,
		AdditionalContext:  result.AdditionalContext,
	}})
}
