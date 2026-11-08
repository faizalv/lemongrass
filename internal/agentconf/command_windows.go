// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build windows

package agentconf

import (
	"path/filepath"
	"strings"
)

// ~ is safe inside a Windows path, where it only appears in 8.3 short names such as RUNNER~1, never at the start.
const windowsSafe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-+@:,=~"

// quoteCommandPath writes the path with forward slashes, since Claude Code runs hook commands through Git Bash, where
// an unquoted backslash is lost, or through cmd.exe, which reads both. A path with only safe characters stays
// unquoted, the one form Git Bash, cmd.exe and PowerShell all run; PowerShell reads a quoted first word as a string, not
// a command. Any other path is double-quoted, which Windows paths allow since they cannot contain a double quote.
func quoteCommandPath(path string) string {
	path = filepath.ToSlash(path)
	if path != "" && strings.Trim(path, windowsSafe) == "" {
		return path
	}
	return `"` + path + `"`
}

// powerShellCommandPath keeps a safe path in the bare forward-slash form every Windows runner reads, so most Codex
// configs stay as earlier keepers wrote them. Any other path becomes a PowerShell call with its native backslashes.
// Only exit 2 makes Codex block a tool call, so a hook PowerShell cannot parse lets every call through unguarded.
func powerShellCommandPath(path string) string {
	slashed := filepath.ToSlash(path)
	if path != "" && strings.Trim(slashed, windowsSafe) == "" {
		return slashed
	}
	return powerShellCall(filepath.FromSlash(path))
}

// cmdCommandPath is the path with forward slashes, escaped for cmd.exe instead of quoted.
func cmdCommandPath(path string) string {
	return caretEscape(filepath.ToSlash(path))
}
