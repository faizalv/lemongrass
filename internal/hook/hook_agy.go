// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/faizalv/lemongrass/internal/guard"
	"github.com/faizalv/lemongrass/internal/session"
)

const agyResumedEnv = "LGRASS_AGY_RESUMED"

type agyHookAdapter struct{}

func (agyHookAdapter) decode(raw []byte) (hookEvent, error) {
	var in struct {
		ConversationID string   `json:"conversationId"`
		WorkspacePaths []string `json:"workspacePaths"`
		TranscriptPath string   `json:"transcriptPath"`
		LastUserInput  string   `json:"lastUserInput"`
		InvocationNum  int      `json:"invocationNum"`
		ToolCall       struct {
			Name string          `json:"name"`
			Args json.RawMessage `json:"args"`
		} `json:"toolCall"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return hookEvent{}, err
	}
	out := hookEvent{SessionID: in.ConversationID, TranscriptPath: in.TranscriptPath, Prompt: in.LastUserInput,
		ToolName: in.ToolCall.Name, ToolInput: in.ToolCall.Args, PreserveSessionState: true,
		Continuation: in.InvocationNum != 0}
	// Hooks run from the config directory; only workspacePaths identifies the launch workspace.
	if len(in.WorkspacePaths) > 0 {
		out.Cwd = in.WorkspacePaths[0]
		out.ProjectDir = in.WorkspacePaths[0]
	}
	var args struct {
		CommandLine string
		Cwd         string
		Action      string
		Input       string
		Path        string
		Content     string
		Target      string
		Replacement string
	}
	if len(in.ToolCall.Args) > 0 {
		if err := json.Unmarshal(in.ToolCall.Args, &args); err != nil {
			return hookEvent{}, err
		}
	}
	var normalized any
	switch out.ToolName {
	case "run_command":
		command := args.CommandLine
		if args.Cwd != "" {
			home, _ := os.UserHomeDir()
			out.Cwd = guard.ResolvePath(args.Cwd, out.ProjectDir, home)
		}
		out.ToolName, normalized = "Bash", shellToolInput{Command: command}
	case "manage_task":
		if args.Action == "send_input" {
			out.ToolName, normalized = "Bash", shellToolInput{Command: args.Input}
		}
	case "write_file":
		out.ToolName, normalized = "Write", writeToolInput{FilePath: args.Path, Content: args.Content}
	case "edit_file":
		out.ToolName, normalized = "Edit", map[string]string{"file_path": args.Path, "old_string": args.Target, "new_string": args.Replacement}
	case "read_file":
		out.ToolName, normalized = "Read", fileToolInput{FilePath: args.Path}
	}
	if normalized != nil {
		out.ToolInput, _ = json.Marshal(normalized)
	}
	return out, nil
}

type agySystemStep struct {
	SystemMessage *agySystemMessage `json:"systemMessage,omitempty"`
	ErrorMessage  *agyErrorMessage  `json:"errorMessage,omitempty"`
}
type agySystemMessage struct {
	Text string `json:"systemMessage"`
}
type agyErrorMessage struct {
	Text string `json:"userMessage"`
}
type agyHookOutput struct {
	Decision            string          `json:"decision,omitempty"`
	Reason              string          `json:"reason,omitempty"`
	InjectSteps         []agySystemStep `json:"injectSteps,omitempty"`
	TerminationBehavior string          `json:"terminationBehavior,omitempty"`
}

func agyOutput(result hookResult) agyHookOutput {
	if result.BlockReason != "" {
		return agyHookOutput{TerminationBehavior: "terminate", InjectSteps: []agySystemStep{{ErrorMessage: &agyErrorMessage{Text: result.BlockReason}}}}
	}
	if result.PermissionDecision == "deny" {
		return agyHookOutput{Decision: "deny", Reason: result.PermissionDecisionReason}
	}
	out := agyHookOutput{}
	if result.AdditionalContext != "" {
		out.InjectSteps = []agySystemStep{{SystemMessage: &agySystemMessage{Text: result.AdditionalContext}}}
		if result.EventName == "PreToolUse" {
			out.Decision = "allow"
		}
	}
	return out
}

func (agyHookAdapter) emit(result hookResult) {
	// agy 1.5.3 treats {} as a denied PreToolUse, whereas empty stdout permits the call.
	if !result.hasOutput() {
		return
	}
	if (result.EventName == "PreToolUse" || result.EventName == "PostToolUse") && result.PermissionDecision != "deny" {
		return
	}
	json.NewEncoder(os.Stdout).Encode(agyOutput(result))
}

func agyOrientation(store *session.Store, tab string) string {
	role := "tab"
	if _, _, ok := liveThinker(store, tab); ok {
		role = "thinker"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, _ := os.ReadFile(filepath.Join(home, ".lemongrass", "orientation", "agy-"+role+".md"))
	return strings.TrimSpace(string(data))
}

func agyResumeLaws(store *session.Store, payload hookEvent) string {
	token := os.Getenv(agyResumedEnv)
	if token == "" || payload.TabID == "" {
		return ""
	}
	mark := "agy-resume:" + token
	marks, err := store.Marks(payload.TabID)
	if err != nil || marks[mark] {
		return ""
	}
	if err := store.MarkReady(payload.TabID, mark); err != nil {
		return ""
	}
	return lawsSummary(payload.Cwd)
}
