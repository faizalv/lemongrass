// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package agentconf

import "strings"

// HookCommand is the command line that runs lgrassdPath with args, in the form the agents' hook runners read on this
// OS. Every hook and statusline command lemongrass registers is built here, so quoting changes in one place.
func HookCommand(lgrassdPath string, args ...string) string {
	return joinCommand(quoteCommandPath(lgrassdPath), args)
}

// CmdHookCommand is HookCommand for a runner that hands the command line to cmd.exe on Windows, as Antigravity does.
// Elsewhere it is the same as HookCommand.
func CmdHookCommand(lgrassdPath string, args ...string) string {
	return joinCommand(cmdCommandPath(lgrassdPath), args)
}

// PowerShellHookCommand is HookCommand for a runner that hands the command line to PowerShell on Windows, as Codex
// does. Elsewhere it is the same as HookCommand.
func PowerShellHookCommand(lgrassdPath string, args ...string) string {
	return joinCommand(powerShellCommandPath(lgrassdPath), args)
}

func joinCommand(path string, args []string) string {
	command := path
	for _, arg := range args {
		command += " " + arg
	}
	return command
}

// cmdSpecial holds the characters cmd.exe splits a line on or acts on when they are outside quotes.
const cmdSpecial = " \t&|<>()^,;="

// caretEscape writes path for cmd.exe with no double quote, putting cmd's escape character before each special
// character. Antigravity runs a command as `cmd /c <command>` through Go's os/exec, which turns every `"` into `\"`;
// cmd keeps those backslashes, so a quoted path is never found. Without a quote Go only wraps the line in one outer
// pair, which cmd strips, and the carets work whether or not the runner escapes the line first.
func caretEscape(path string) string {
	var b strings.Builder
	for _, r := range path {
		if strings.ContainsRune(cmdSpecial, r) {
			b.WriteByte('^')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// powerShellQuotes are the characters PowerShell ends a single-quoted string on: the ASCII quote and its typographic
// forms. Each is escaped by doubling it.
const powerShellQuotes = "'‘’‚‛"

// powerShellCall writes path as a PowerShell call of a single-quoted string. Codex runs a hook as
// `powershell -Command <command>` on Windows, and PowerShell reads a quoted first word as a string rather than a
// command, so a quoted path only runs behind the call operator. Inside single quotes nothing else is special.
func powerShellCall(path string) string {
	var b strings.Builder
	b.WriteString("& '")
	for _, r := range path {
		if strings.ContainsRune(powerShellQuotes, r) {
			b.WriteRune(r)
		}
		b.WriteRune(r)
	}
	b.WriteString("'")
	return b.String()
}
