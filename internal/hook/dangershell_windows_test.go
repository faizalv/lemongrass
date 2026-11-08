// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"encoding/json"
	"testing"
)

func TestWindowsShellToolCommand(t *testing.T) {
	cases := []struct {
		name    string
		tool    string
		vendor  string
		input   string
		command string
		judge   string
		denied  bool
		isShell bool
	}{
		{"claude bash is judged as is", "Bash", "claude", `{"command":"rm -rf x"}`, "rm -rf x", "Bash", false, true},
		{"codex bash string runs powershell", "Bash", "codex", `{"command":"Remove-Item x"}`, "Remove-Item x", "PowerShell", false, true},
		{"agy normalized bash runs powershell", "Bash", "agy", `{"command":"Remove-Item x"}`, "Remove-Item x", "PowerShell", false, true},
		{"agy normalized bash explicitly starts bash", "Bash", "agy", `{"command":"bash -lc 'ls'"}`, "bash -lc 'ls'", "Bash", false, true},
		{"codex bash string starting bash", "Bash", "codex", `{"command":"bash -lc 'ls'"}`, "bash -lc 'ls'", "Bash", false, true},
		{"codex git bash by full path", "Bash", "codex", `{"command":"& 'C:\\Program Files\\Git\\bin\\bash.exe' -c 'rm -rf x'"}`, "rm -rf x", "Bash", false, true},
		{"agy git bash by full path", "Bash", "agy", `{"command":"& 'C:\\Program Files\\Git\\bin\\bash.exe' -lc 'git status'"}`, "git status", "Bash", false, true},
		{"git bash by full path with a double quote", "Bash", "codex", `{"command":"& 'C:\\Git\\bin\\bash.exe' -c 'echo \"x y\"'"}`, "", "", true, true},
		{"git bash by full path then a statement", "Bash", "codex", `{"command":"& 'C:\\Git\\bin\\bash.exe' -c 'ls -la'; Remove-Item x"}`, `& 'C:\Git\bin\bash.exe' -c 'ls -la'; Remove-Item x`, "PowerShell", false, true},
		{"codex shell argv with bash", "shell", "codex", `{"command":["bash","-lc","ls -la"]}`, "ls -la", "Bash", false, true},
		{"codex shell argv with git bash path", "shell", "codex", `{"command":["C:\\Program Files\\Git\\bin\\bash.exe","-c","ls"]}`, "ls", "Bash", false, true},
		{"codex shell argv with powershell", "shell", "codex", `{"command":["powershell.exe","-Command","ls"]}`, "'powershell.exe' '-Command' 'ls'", "Bash", false, true},
		{"wsl bash argv goes to the guard", "shell", "codex", `{"command":["C:\\Windows\\System32\\bash.exe","-c","ls"]}`, `'C:\Windows\System32\bash.exe' '-c' 'ls'`, "Bash", false, true},
		{"exec_command with a bash shell", "exec_command", "codex", `{"cmd":"ls","shell":"bash"}`, "ls", "Bash", false, true},
		{"exec_command with a powershell shell", "exec_command", "codex", `{"cmd":"Get-Date","shell":"powershell"}`, "Get-Date", "PowerShell", false, true},
		{"exec_command with the codex prelude", "exec_command", "codex", `{"cmd":"try { [Console]::OutputEncoding=[System.Text.Encoding]::UTF8 } catch {}; Get-Date","shell":"C:\\Program Files\\PowerShell\\7\\pwsh.exe"}`, " Get-Date", "PowerShell", false, true},
		{"exec_command with a cmd shell", "exec_command", "codex", `{"cmd":"dir","shell":"cmd"}`, "", "", true, true},
		{"exec_command in the default shell", "exec_command", "codex", `{"cmd":"dir"}`, "dir", "PowerShell", false, true},
		{"powershell tool", "PowerShell", "claude", `{"command":"Get-ChildItem"}`, "Get-ChildItem", "PowerShell", false, true},
		{"prelude kept outside codex", "PowerShell", "claude", `{"command":"try { [Console]::OutputEncoding=[System.Text.Encoding]::UTF8 } catch {}; x"}`, "try { [Console]::OutputEncoding=[System.Text.Encoding]::UTF8 } catch {}; x", "PowerShell", false, true},
		{"no command", "PowerShell", "claude", `{"command":""}`, "", "", true, true},
		{"unreadable input", "shell", "codex", `{"command":42}`, "", "", true, true},
		{"read tool is not a shell", "Read", "claude", `{"file_path":"x"}`, "", "", false, false},
	}
	for _, tc := range cases {
		payload := hookEvent{ToolName: tc.tool, Vendor: tc.vendor, ToolInput: json.RawMessage(tc.input)}
		command, judge, verdict, isShell := shellToolCommand(payload)
		if command != tc.command || judge != tc.judge || (verdict != nil) != tc.denied || isShell != tc.isShell {
			t.Errorf("%s: got %q as %q, denied %v, shell %v", tc.name, command, judge, verdict != nil, isShell)
		}
		if verdict != nil && verdict.RuleID != "opaque-shell" {
			t.Errorf("%s: denied by %s, want opaque-shell", tc.name, verdict.RuleID)
		}
	}
}
