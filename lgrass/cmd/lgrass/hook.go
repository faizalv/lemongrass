package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
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
		store.Start(payload.SessionID, os.Getenv("CLAUDE_CODE_MESSAGING_SOCKET"), os.Getenv("CLAUDE_CODE_MESSAGING_TOKEN"))
	case "SessionEnd":
		store.End(payload.SessionID)
	case "PreToolUse":
		hookPreToolUse(store, payload)
	case "PostToolUse":
		hookPostToolUse(store, payload)
	}
}

func hookPreToolUse(store *session.Store, payload hookPayload) {
	var parts []string

	if payload.ToolName == "Write" || payload.ToolName == "Edit" {
		if filePath := toolFilePath(payload); filePath != "" {
			if hits, err := store.RecentActivity(payload.SessionID, filePath, collisionWindow); err == nil && len(hits) > 0 {
				if w := session.FormatCollisionWarning(hits); w != "" {
					parts = append(parts, w)
				}
			}
		}
	}
	parts = append(parts, mentionContext(store, payload.SessionID)...)

	emitHookContext("PreToolUse", "allow", parts)
}

func hookPostToolUse(store *session.Store, payload hookPayload) {
	store.Touch(payload.SessionID)

	if payload.ToolName == "Write" || payload.ToolName == "Edit" {
		if filePath := toolFilePath(payload); filePath != "" {
			store.LogFileActivity(payload.SessionID, filePath)
		}
	}

	var parts []string
	parts = append(parts, mentionContext(store, payload.SessionID)...)

	if fire, err := store.IncrementNudgeCounter(payload.SessionID, nudgeThreshold); err == nil && fire {
		if liveness, err := store.Liveness(payload.SessionID, idleThreshold); err == nil {
			parts = append(parts, session.FormatNudge(liveness))
		}
	}

	emitHookContext("PostToolUse", "", parts)
}

// mentionContext is the pull-based fallback for thread @mentions,
// checked on every hook firing (not throttled like the nudge) so a
// mention surfaces at the next tool-call boundary regardless of whether
// deliver.go's live socket push reached this session. Marks them read
// once surfaced, so the same mention doesn't repeat on the next hook.
func mentionContext(store *session.Store, sessionID string) []string {
	msgs, err := store.UnreadMentions(sessionID)
	if err != nil || len(msgs) == 0 {
		return nil
	}
	text := session.FormatMentions(msgs)
	if text == "" {
		return nil
	}
	store.MarkThreadRead(sessionID)
	return []string{text}
}

func toolFilePath(payload hookPayload) string {
	var input fileToolInput
	if err := json.Unmarshal(payload.ToolInput, &input); err != nil {
		return ""
	}
	return input.FilePath
}

// emitHookContext joins non-empty parts (collision warning, mentions,
// nudge, whichever fired this call) into one additionalContext string,
// since a hook response carries only one, and emits nothing at all when
// none fired.
func emitHookContext(eventName, permissionDecision string, parts []string) {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) == 0 {
		return
	}
	json.NewEncoder(os.Stdout).Encode(hookOutput{HookSpecificOutput: hookSpecificOutput{
		HookEventName:      eventName,
		PermissionDecision: permissionDecision,
		AdditionalContext:  strings.Join(nonEmpty, "\n\n"),
	}})
}
