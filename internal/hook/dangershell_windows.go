// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build windows

package hook

import (
	"encoding/json"
	"strings"

	"github.com/faizalv/lemongrass/internal/gitbash"
	"github.com/faizalv/lemongrass/internal/guard"
)

// windowsShellTools are the tool names that run a shell command on Windows: Claude's Bash and PowerShell tools and Codex's shell tools.
var windowsShellTools = map[string]bool{"Bash": true, "PowerShell": true, "shell": true, "local_shell": true, "exec_command": true, "container.exec": true}

// shellToolCommand returns the command text the gate judges for a shell tool call, the guard tool that reads it
// ("Bash" or "PowerShell"), and isShell false for any other tool. Claude's Bash tool runs through Git Bash, so its
// command is judged as bash. Codex and agy shell calls and Claude's PowerShell tool run PowerShell unless they name a
// POSIX shell with -c, so their text is judged as PowerShell; an argv is judged as one command line, whose pwsh and
// powershell hosts the guard reads; cmd is denied as opaque-shell.
func shellToolCommand(payload hookEvent) (command, tool string, verdict *guard.Verdict, isShell bool) {
	if !windowsShellTools[payload.ToolName] {
		return "", "", nil, false
	}
	var input struct {
		Command json.RawMessage `json:"command"`
		Cmd     string          `json:"cmd"`
		Shell   string          `json:"shell"`
	}
	if err := json.Unmarshal(payload.ToolInput, &input); err != nil {
		return "", "", guard.OpaqueShellVerdict("The tool input could not be read."), true
	}
	var text string
	var argv []string
	if len(input.Command) > 0 && json.Unmarshal(input.Command, &text) != nil {
		if json.Unmarshal(input.Command, &argv) != nil {
			return "", "", guard.OpaqueShellVerdict("The tool input could not be read."), true
		}
	}
	if payload.ToolName == "Bash" && !isCodexPayload(payload) && payload.Vendor != "agy" {
		return text, "Bash", nil, true
	}
	switch {
	case len(argv) > 0:
		if script, ok := posixScript(argv); ok {
			return script, "Bash", nil, true
		}
		return argvCommandLine(argv), "Bash", nil, true
	case input.Cmd != "" && input.Shell != "":
		if posixShellName(input.Shell) {
			if script, ok := posixScript([]string{input.Shell, "-c", input.Cmd}); ok {
				return script, "Bash", nil, true
			}
			return "", "", guard.OpaqueShellVerdict("The tool input could not be read."), true
		}
		if powerShellHostName(input.Shell) {
			return powerShellText(payload, input.Cmd), "PowerShell", nil, true
		}
		return "", "", guard.OpaqueShellVerdict(opaqueShellHint(payload.ToolName, input.Shell)), true
	case text != "" || input.Cmd != "":
		line := text
		if line == "" {
			line = input.Cmd
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 && posixShellName(fields[0]) && strings.HasPrefix(fields[1], "-") && strings.Contains(fields[1], "c") {
			return line, "Bash", nil, true
		}
		if script, refusal, ok := powerShellPosixScript(line); ok {
			if refusal != "" {
				return "", "", guard.OpaqueShellVerdict(refusal), true
			}
			return script, "Bash", nil, true
		}
		return powerShellText(payload, line), "PowerShell", nil, true
	}
	return "", "", guard.OpaqueShellVerdict("The tool input holds no command."), true
}

// codexPrelude is the statement Codex puts before every PowerShell command it runs, so output reaches it as UTF-8.
// It sets a .NET static property, which the judge refuses, so it is removed when it is exactly this leading text.
const codexPrelude = "try { [Console]::OutputEncoding=[System.Text.Encoding]::UTF8 } catch {}"

// powerShellText is the PowerShell the gate judges: the text as the tool runs it, less Codex's own prelude.
func powerShellText(payload hookEvent, text string) string {
	if !isCodexPayload(payload) || !strings.HasPrefix(text, codexPrelude) {
		return text
	}
	rest := strings.TrimLeft(strings.TrimPrefix(text, codexPrelude), " \t")
	return strings.TrimPrefix(rest, ";")
}

// argvCommandLine writes an argv as one shell command line, each word single-quoted, so the guard reads it as the
// program and arguments the tool starts.
func argvCommandLine(argv []string) string {
	words := make([]string, len(argv))
	for i, word := range argv {
		words[i] = shellSingleQuote(word)
	}
	return strings.Join(words, " ")
}

// opaqueShellHint answers a call in a shell the guard cannot read, such as cmd, with the two it can.
func opaqueShellHint(tool, shell string) string {
	hint := "On Windows this " + tool + " call runs " + shell + ", whose text the guard cannot read. Run it as PowerShell, or through Git Bash: bash -c '<command>'."
	if bash, err := gitbash.Bash(); err == nil {
		hint += " If bash is not Git Bash there, run & " + psSingleQuote(bash) + " -c '<command>', with each ' inside <command> written as '' and no double quote."
	}
	return hint
}

// posixScript returns the script of an argv that runs a POSIX shell with -c, such as ["bash", "-lc", "ls"].
func posixScript(argv []string) (string, bool) {
	if len(argv) < 3 || !posixShellName(argv[0]) {
		return "", false
	}
	for i := 1; i < len(argv)-1; i++ {
		flag := argv[i]
		if !strings.HasPrefix(flag, "-") || strings.HasPrefix(flag, "--") {
			return "", false
		}
		if strings.Contains(flag, "c") {
			return argv[i+1], true
		}
	}
	return "", false
}

func isCodexPayload(payload hookEvent) bool {
	return payload.Vendor == "codex"
}
