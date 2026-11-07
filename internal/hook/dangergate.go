// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/faizalv/lemongrass/internal/guard"
	"github.com/faizalv/lemongrass/internal/project"
)

const (
	// launchProjectEnv is set by lemongrass on every agent tab it spawns, whatever the vendor.
	launchProjectEnv    = "LGRASS_PROJECT_DIR"
	claudeProjectDirEnv = "CLAUDE_PROJECT_DIR"
)

func dangerDecision(payload hookEvent) *guard.Verdict {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	launch := launchProject()
	input := guard.Input{Tool: payload.ToolName, Cwd: payload.Cwd, Home: home, ProjectRoot: launch.Path}
	if command, verdict, isShell := shellToolCommand(payload); isShell {
		if verdict != nil {
			return verdict
		}
		input.Tool = "Bash"
		input.Command = command
	} else {
		input.Paths = dangerToolPaths(payload)
		input.Writes = isMemoryWriteTool(payload.ToolName)
	}
	policy := hookPolicy(launch.ID)
	verdict := guard.Decide(input, policy)
	if verdict != nil && verdict.Mode == guard.ModeDeny {
		return verdict
	}
	if deletion := patchDeletionVerdict(payload, input, policy); deletion != nil && (verdict == nil || deletion.Mode == guard.ModeDeny) {
		return deletion
	}
	return verdict
}

// patchDeletionVerdict judges each file an apply_patch deletes as the guard judges `rm -- <path>`, so a patch is held to the same deletion rules and launch project exemption as a shell rm.
func patchDeletionVerdict(payload hookEvent, input guard.Input, policy guard.Policy) *guard.Verdict {
	deleted := patchDeletedPaths(payload)
	if len(deleted) == 0 {
		return nil
	}
	quoted := make([]string, len(deleted))
	for i, p := range deleted {
		quoted[i] = shellSingleQuote(p)
	}
	input.Tool = "Bash"
	input.Paths = nil
	input.Writes = false
	input.Command = "rm -- " + strings.Join(quoted, " ")
	return guard.Decide(input, policy)
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func dangerToolPaths(payload hookEvent) []string {
	paths := memoryToolPaths(payload)
	var generic map[string]any
	if err := json.Unmarshal(payload.ToolInput, &generic); err != nil {
		return paths
	}
	keys := []string{"path", "glob"}
	if payload.ToolName == "Glob" {
		keys = append(keys, "pattern")
	}
	for _, key := range keys {
		if text, ok := generic[key].(string); ok && text != "" {
			paths = append(paths, text)
		}
	}
	return paths
}

// launchProject is the registered project the agent was started in, which a cd inside the session does not move. It is the zero Project when there is none. CLAUDE_PROJECT_DIR covers a Claude Code session started outside lemongrass.
func launchProject() project.Project {
	dir := os.Getenv(launchProjectEnv)
	if dir == "" {
		dir = os.Getenv(claudeProjectDirEnv)
	}
	if dir == "" {
		return project.Project{}
	}
	proj, err := project.Resolve(dir)
	if err != nil {
		return project.Project{}
	}
	return proj
}
