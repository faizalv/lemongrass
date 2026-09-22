package main

import (
	"bufio"
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
type hookEvent struct {
	SessionID            string          `json:"session_id"`
	TranscriptPath       string          `json:"transcript_path"`
	Cwd                  string          `json:"cwd"`
	ToolName             string          `json:"tool_name"`
	ToolInput            json.RawMessage `json:"tool_input"`
	SessionSource        string          `json:"source"`
	MessagingSocket      string
	MessagingToken       string
	EnforceBibliothek    bool
	PreserveSessionState bool
}

type fileToolInput struct {
	FilePath string `json:"file_path"`
}

type skillToolInput struct {
	Skill string `json:"skill"`
}

type patchToolInput struct {
	Command string `json:"command"`
}

const bibliothekChecklistID = "bibliothek"

// Long enough to outlast any real session.
const bibliothekSignatureTTL = 7 * 24 * time.Hour

// Every failure here fails soft so a hook-side problem never breaks the agent's own hook chain.
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
	adapter := hookAdapterForEnvironment()
	payload, err := adapter.decode(raw)
	if err != nil {
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

	var result hookResult
	switch event {
	case "SessionStart":
		result = hookSessionStart(store, payload, proj.Path)
	case "SessionEnd":
		store.End(payload.SessionID)
	case "PreToolUse":
		result = hookPreToolUse(store, payload, proj.Path)
	case "PostToolUse":
		result = hookPostToolUse(store, payload, proj.Path)
	}
	adapter.emit(result)
}

func hookSessionStart(store *session.Store, payload hookEvent, projectPath string) hookResult {
	if payload.PreserveSessionState {
		store.EnsureOpen(payload.SessionID)
	} else {
		store.Start(payload.SessionID, payload.MessagingSocket, payload.MessagingToken)
	}

	var parts []string
	if summary := lawsSummary(projectPath); summary != "" {
		parts = append(parts, summary)
	}
	return newHookResult("SessionStart", "", parts)
}

// Missing biblio/laws/summary.md is not an error -- most projects have no laws to deliver.
func lawsSummary(projectPath string) string {
	data, err := os.ReadFile(filepath.Join(projectPath, "biblio", "laws", "summary.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func hookPreToolUse(store *session.Store, payload hookEvent, projectPath string) hookResult {
	filePaths := toolFilePaths(payload)

	if payload.EnforceBibliothek && hasBiblio(projectPath) {
		if isBibliothekSkillCall(payload) {
			store.Sign(payload.SessionID, bibliothekChecklistID)
		} else if deny := bibliothekDeny(store, payload.SessionID, payload.TranscriptPath); deny != "" {
			return newHookResult("PreToolUse", "deny", []string{deny})
		}
	}

	if deny := checklistDeny(store, payload, filePaths, projectPath); deny != "" {
		return newHookResult("PreToolUse", "deny", []string{deny})
	}

	var parts []string

	if isFileEdit(payload) {
		for _, filePath := range filePaths {
			if hits, err := store.RecentActivity(payload.SessionID, filePath, collisionWindow); err == nil && len(hits) > 0 {
				if w := session.FormatCollisionWarning(hits); w != "" {
					parts = append(parts, w)
				}
			}
		}
	}
	parts = append(parts, mentionContext(store, payload.SessionID)...)

	return newHookResult("PreToolUse", "allow", parts)
}

func isBibliothekSkillCall(payload hookEvent) bool {
	if payload.ToolName != "Skill" {
		return false
	}
	var input skillToolInput
	if err := json.Unmarshal(payload.ToolInput, &input); err != nil {
		return false
	}
	return input.Skill == "bibliothek"
}

// Returns the deny message when this session hasn't signed the bibliothek gate yet (or its signature expired), "" once signed.
func bibliothekDeny(store *session.Store, sessionID, transcriptPath string) string {
	signedAt, err := store.SignedAt(sessionID, bibliothekChecklistID)
	if err == nil && !signedAt.IsZero() && time.Since(signedAt) <= bibliothekSignatureTTL {
		return ""
	}

	// Not signed for this session yet -- but the harness can dedup a repeat
	// Skill(bibliothek) call into no fresh tool-call cycle (already loaded,
	// instructions unchanged), so the auto-sign in hookPreToolUse never runs.
	// The transcript itself still carries the original call regardless, so
	// check it directly before denying.
	if transcriptHasBibliothekInvocation(transcriptPath) {
		store.Sign(sessionID, bibliothekChecklistID)
		return ""
	}

	return session.FormatBibliothekDeny()
}

type transcriptToolUse struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Input struct {
		Skill string `json:"skill"`
	} `json:"input"`
}

type transcriptEntry struct {
	Message struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func transcriptHasBibliothekInvocation(transcriptPath string) bool {
	if transcriptPath == "" {
		return false
	}
	f, err := os.Open(transcriptPath)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		var entry transcriptEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		var blocks []transcriptToolUse
		if err := json.Unmarshal(entry.Message.Content, &blocks); err != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type == "tool_use" && b.Name == "Skill" && b.Input.Skill == "bibliothek" {
				return true
			}
		}
	}
	return false
}

// Returns the deny message for the first unsigned or TTL-expired checklist matching this call, "" if none match or all are signed.
func checklistDeny(store *session.Store, payload hookEvent, filePaths []string, projectPath string) string {
	checklists, err := session.LoadChecklists(projectPath)
	if err != nil || len(checklists) == 0 {
		return ""
	}
	if len(filePaths) == 0 {
		filePaths = []string{""}
	}
	for _, c := range checklists {
		if !checklistMatches(c, payload.ToolName, filePaths) {
			continue
		}
		signedAt, err := store.SignedAt(payload.SessionID, c.ID)
		if err != nil {
			continue
		}
		if signedAt.IsZero() || time.Since(signedAt) > c.TTL() {
			if payload.PreserveSessionState {
				return fmt.Sprintf("lgrass: %q requires signing before this call proceeds. Run `lgrass sign --session-id %s %s`, then retry:\n\n%s", c.ID, payload.SessionID, c.ID, c.Content)
			}
			return session.FormatChecklistDeny(c)
		}
	}
	return ""
}

func hookPostToolUse(store *session.Store, payload hookEvent, projectPath string) hookResult {
	store.Touch(payload.SessionID)

	if isFileEdit(payload) {
		for _, filePath := range toolFilePaths(payload) {
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

	return newHookResult("PostToolUse", "", parts)
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

func hookAdapterForEnvironment() hookAdapter {
	if os.Getenv("LGRASS_HOOK_VENDOR") == "codex" {
		return codexHookAdapter{}
	}
	return claudeHookAdapter{}
}

type hookAdapter interface {
	decode([]byte) (hookEvent, error)
	emit(hookResult)
}

func isFileEdit(payload hookEvent) bool {
	return payload.ToolName == "Write" || payload.ToolName == "Edit" || payload.ToolName == "apply_patch"
}

func checklistMatches(checklist session.Checklist, toolName string, filePaths []string) bool {
	for _, filePath := range filePaths {
		if checklist.Matches(toolName, filePath) {
			return true
		}
		if toolName == "apply_patch" && (checklist.Matches("Write", filePath) || checklist.Matches("Edit", filePath)) {
			return true
		}
	}
	return false
}

func toolFilePaths(payload hookEvent) []string {
	var input fileToolInput
	if err := json.Unmarshal(payload.ToolInput, &input); err == nil && input.FilePath != "" {
		return []string{input.FilePath}
	}
	if payload.ToolName != "apply_patch" {
		return nil
	}
	var patch patchToolInput
	if err := json.Unmarshal(payload.ToolInput, &patch); err != nil {
		return nil
	}
	return patchFilePaths(patch.Command)
}

func patchFilePaths(command string) []string {
	prefixes := []string{"*** Add File: ", "*** Delete File: ", "*** Update File: "}
	seen := map[string]bool{}
	var paths []string
	for _, line := range strings.Split(command, "\n") {
		for _, prefix := range prefixes {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			path := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if path != "" && !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
			break
		}
	}
	return paths
}
