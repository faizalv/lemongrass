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

// shellToolCommand returns the shell text the gate judges for a shell tool call, and isShell false for any other tool. Claude's Bash tool runs through Git Bash, so its command is judged as is. Every other shell call on Windows is judged only when it names a POSIX shell with -c explicitly, since Codex and the PowerShell tool run PowerShell by default, whose text the POSIX parser misreads; anything else is denied as opaque-shell.
func shellToolCommand(payload hookEvent) (command string, verdict *guard.Verdict, isShell bool) {
	if !windowsShellTools[payload.ToolName] {
		return "", nil, false
	}
	var input struct {
		Command json.RawMessage `json:"command"`
		Cmd     string          `json:"cmd"`
		Shell   string          `json:"shell"`
	}
	if err := json.Unmarshal(payload.ToolInput, &input); err != nil {
		return "", guard.OpaqueShellVerdict("The tool input could not be read."), true
	}
	var text string
	var argv []string
	if len(input.Command) > 0 && json.Unmarshal(input.Command, &text) != nil {
		if json.Unmarshal(input.Command, &argv) != nil {
			return "", guard.OpaqueShellVerdict("The tool input could not be read."), true
		}
	}
	if payload.ToolName == "Bash" && !isCodexPayload(payload) && payload.Vendor != "agy" {
		return text, nil, true
	}
	switch {
	case len(argv) > 0:
		if script, ok := posixScript(argv); ok {
			return script, nil, true
		}
	case input.Cmd != "" && input.Shell != "":
		if script, ok := posixScript([]string{input.Shell, "-c", input.Cmd}); ok {
			return script, nil, true
		}
	case text != "" || input.Cmd != "":
		// A command string runs in the tool's default shell, PowerShell on Windows, unless it starts bash itself; the parser unwraps that bash -c.
		line := text
		if line == "" {
			line = input.Cmd
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 && posixShellName(fields[0]) && strings.HasPrefix(fields[1], "-") && strings.Contains(fields[1], "c") {
			return line, nil, true
		}
		if script, refusal, ok := powerShellPosixScript(line); ok {
			if refusal != "" {
				return "", guard.OpaqueShellVerdict(refusal), true
			}
			return script, nil, true
		}
	}
	return "", guard.OpaqueShellVerdict(opaqueShellHint(payload.ToolName)), true
}

// opaqueShellHint names both routes into Git Bash. Lemongrass puts Git Bash first on PATH in the Codex and
// Antigravity tabs it starts, so bare bash is Git Bash there; where it is not, bare bash from PowerShell is
// System32's WSL launcher, so the full path is named in the call-operator form powerShellPosixScript reads.
func opaqueShellHint(tool string) string {
	hint := "On Windows a " + tool + " call runs PowerShell or cmd, whose text the guard cannot read. Run it through Git Bash instead: bash -c '<command>'."
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
