package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/project"
	"github.com/faizalv/lemongrass/session"
)

const (
	collisionWindow = 15 * time.Minute
	idleThreshold   = 5 * time.Minute
	nudgeThreshold  = 8 // tool calls between periodic nudges
)

// The full hook JSON carries more fields depending on hook_event_name; this is only the subset lgrass reads.
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

// Every failure here fails soft (exit 0, no output) so an lgrass-side problem never breaks Claude Code's own hook chain.
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
		hookSessionStart(store, payload, proj.Path)
	case "SessionEnd":
		store.End(payload.SessionID)
	case "PreToolUse":
		hookPreToolUse(store, payload, proj.Path)
	case "PostToolUse":
		hookPostToolUse(store, payload, proj.Path)
	}
}

func hookSessionStart(store *session.Store, payload hookPayload, projectPath string) {
	store.Start(payload.SessionID, os.Getenv("CLAUDE_CODE_MESSAGING_SOCKET"), os.Getenv("CLAUDE_CODE_MESSAGING_TOKEN"))

	var parts []string
	if summary := lawsSummary(projectPath); summary != "" {
		parts = append(parts, summary)
	}
	emitHookContext("SessionStart", "", parts)
}

// Missing biblio/laws/summary.md is not an error -- most projects have no laws to deliver.
func lawsSummary(projectPath string) string {
	data, err := os.ReadFile(filepath.Join(projectPath, "biblio", "laws", "summary.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func hookPreToolUse(store *session.Store, payload hookPayload, projectPath string) {
	filePath := toolFilePath(payload)

	if deny := checklistDeny(store, payload, filePath, projectPath); deny != "" {
		emitHookContext("PreToolUse", "deny", []string{deny})
		return
	}

	var parts []string

	if payload.ToolName == "Write" || payload.ToolName == "Edit" {
		if filePath != "" {
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

// Returns the deny message for the first unsigned or TTL-expired checklist matching this call, "" if none match or all are signed.
func checklistDeny(store *session.Store, payload hookPayload, filePath, projectPath string) string {
	checklists, err := session.LoadChecklists(projectPath)
	if err != nil || len(checklists) == 0 {
		return ""
	}
	for _, c := range checklists {
		if !c.Matches(payload.ToolName, filePath) {
			continue
		}
		signedAt, err := store.SignedAt(payload.SessionID, c.ID)
		if err != nil {
			continue
		}
		if signedAt.IsZero() || time.Since(signedAt) > c.TTL() {
			return session.FormatChecklistDeny(c)
		}
	}
	return ""
}

func hookPostToolUse(store *session.Store, payload hookPayload, projectPath string) {
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
		if hasBiblio(projectPath) {
			var userTips []string
			if tips, err := store.ListTips(); err == nil {
				for _, t := range tips {
					userTips = append(userTips, t.Message)
				}
			}
			parts = append(parts, session.RandomTipFrom(userTips))
		}
	}

	emitHookContext("PostToolUse", "", parts)
}

func hasBiblio(projectPath string) bool {
	info, err := os.Stat(filepath.Join(projectPath, "biblio"))
	return err == nil && info.IsDir()
}

// Unthrottled unlike the nudge, so a mention surfaces at the next tool call regardless of whether the live socket push reached this session.
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

// A hook response carries only one additionalContext string, so this joins whatever parts fired into one.
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
