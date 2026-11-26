// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"os"

	"github.com/faizalv/lemongrass/internal/guard"
	"github.com/faizalv/lemongrass/internal/project"
)

// A tab's own state belongs to the project it was launched in, while a call may run in or name another project: a
// Codex shell call sends its workdir as the payload cwd, and a Claude command carries `cd /other && ...` in its
// text. So the store follows the tab, and the gates judge the call by every project it reaches.

// storeProject picks the project whose store holds this call's state. A lemongrass tab uses its launch project, so
// no cwd moves its tab state, session, notifications or gate marks; a session without a tab resolves from the cwd.
func storeProject(payload hookEvent, projectDir string) (project.Project, error) {
	if payload.TabID != "" {
		if launch := tabLaunchProject(payload); launch.Path != "" {
			return launch, nil
		}
	}
	return project.Resolve(projectDir)
}

// tabLaunchProject is launchProject with Antigravity's first workspace as the fallback, since agy hooks carry the
// workspace in the payload.
func tabLaunchProject(payload hookEvent) project.Project {
	launch := launchProject()
	if launch.Path == "" && payload.Vendor == "agy" && payload.ProjectDir != "" {
		launch, _ = project.Resolve(payload.ProjectDir)
	}
	return launch
}

// callInput reduces a tool call to the guard's input with the launch root as ProjectRoot. A shell tool whose input
// cannot be read returns its verdict instead.
func callInput(payload hookEvent, launchRoot string) (guard.Input, *guard.Verdict) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	input := guard.Input{Tool: payload.ToolName, Cwd: payload.Cwd, Home: home, ProjectRoot: launchRoot}
	if command, tool, verdict, isShell := shellToolCommand(payload); isShell {
		if verdict != nil {
			return input, verdict
		}
		input.Tool = tool
		input.Command = command
		return input, nil
	}
	input.Paths = dangerToolPaths(payload)
	input.Writes = isMemoryWriteTool(payload.ToolName)
	return input, nil
}

// touchedProjects returns the registered projects other than base that the call names literally.
func touchedProjects(input guard.Input, base project.Project) []project.Project {
	var out []project.Project
	for _, p := range project.ResolveMany(guard.TouchedPaths(input)) {
		if p.ID != base.ID {
			out = append(out, p)
		}
	}
	return out
}

// callRoots is projectPath followed by every other registered project the call reaches, for the gates and the
// shell write log.
func callRoots(payload hookEvent, projectPath string) []string {
	roots := []string{projectPath}
	if projectPath == "" {
		return roots
	}
	base, err := project.Resolve(projectPath)
	if err != nil {
		return roots
	}
	input, _ := callInput(payload, projectPath)
	for _, p := range touchedProjects(input, base) {
		roots = append(roots, p.Path)
	}
	return roots
}

func anyBiblio(roots []string) bool {
	for _, root := range roots {
		if hasBiblio(root) {
			return true
		}
	}
	return false
}
