package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/faizalv/lemongrass/project"
	"github.com/faizalv/lemongrass/session"
)

// Tunable defaults, adjustable independently of everything else here.
const (
	collisionWindow = 15 * time.Minute
	idleThreshold   = 5 * time.Minute
	nudgeThreshold  = 8 // tool calls between periodic nudges
)

// hookPayload is the subset of Claude Code's hook JSON (on stdin) that
// lgrass cares about. The full payload carries more fields depending on
// hook_event_name, all ignored here.
type hookPayload struct {
	SessionID string          `json:"session_id"`
	Cwd       string          `json:"cwd"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

type fileToolInput struct {
	FilePath string `json:"file_path"`
}

type hookOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

type hookSpecificOutput struct {
	HookEventName      string `json:"hookEventName"`
	PermissionDecision string `json:"permissionDecision,omitempty"`
	AdditionalContext  string `json:"additionalContext,omitempty"`
}

// cmdHook implements `lgrass hook <event>`, invoked by Claude Code's own
// hook system with the event JSON on stdin. Every failure mode here
// (unparseable JSON, an unregistered project, a db error) fails soft:
// exit 0 with no output, never breaking the hook chain over an
// lgrass-side issue.
func cmdHook(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: lgrass hook <SessionStart|SessionEnd|PreToolUse|PostToolUse>")
		os.Exit(1)
	}
	event := args[0]

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(0)
	}
	var payload hookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		os.Exit(0)
	}

	proj, err := project.Resolve(payload.Cwd)
	if err != nil {
		os.Exit(0)
	}

	store, err := session.Open(session.DBPath(), proj.ID)
	if err != nil {
		os.Exit(0)
	}
	defer store.Close()

	switch event {
	case "SessionStart":
		store.Start(payload.SessionID)
	case "SessionEnd":
		store.End(payload.SessionID)
	case "PreToolUse":
		hookPreToolUse(store, payload)
	case "PostToolUse":
		hookPostToolUse(store, payload)
	}
}

func hookPreToolUse(store *session.Store, payload hookPayload) {
	if payload.ToolName != "Write" && payload.ToolName != "Edit" {
		return
	}
	filePath := toolFilePath(payload)
	if filePath == "" {
		return
	}

	hits, err := store.RecentActivity(payload.SessionID, filePath, collisionWindow)
	if err != nil || len(hits) == 0 {
		return
	}
	warning := session.FormatCollisionWarning(hits)
	if warning == "" {
		return
	}
	emitHookOutput(hookSpecificOutput{
		HookEventName:      "PreToolUse",
		PermissionDecision: "allow",
		AdditionalContext:  warning,
	})
}

func hookPostToolUse(store *session.Store, payload hookPayload) {
	store.Touch(payload.SessionID)

	if payload.ToolName == "Write" || payload.ToolName == "Edit" {
		if filePath := toolFilePath(payload); filePath != "" {
			store.LogFileActivity(payload.SessionID, filePath)
		}
	}

	fire, err := store.IncrementNudgeCounter(payload.SessionID, nudgeThreshold)
	if err != nil || !fire {
		return
	}
	liveness, err := store.Liveness(payload.SessionID, idleThreshold)
	if err != nil {
		return
	}
	emitHookOutput(hookSpecificOutput{
		HookEventName:     "PostToolUse",
		AdditionalContext: session.FormatNudge(liveness),
	})
}

func toolFilePath(payload hookPayload) string {
	var input fileToolInput
	if err := json.Unmarshal(payload.ToolInput, &input); err != nil {
		return ""
	}
	return input.FilePath
}

func emitHookOutput(out hookSpecificOutput) {
	json.NewEncoder(os.Stdout).Encode(hookOutput{HookSpecificOutput: out})
}
