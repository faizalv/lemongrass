// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"os"
	"path/filepath"
	"testing"
)

// writePs1 writes a script into a fresh directory and returns the directory, used as the call's cwd.
func writePs1(t *testing.T, name, text string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPowerShellScriptScan(t *testing.T) {
	cases := []struct {
		name, script, tool, command, want string
	}{
		{"bash runs a ps1 by path", "iex 'x'", "Bash", "./x.ps1", "script-command"},
		{"pwsh -File", "iex 'x'", "Bash", "pwsh -NoProfile -File x.ps1 arg", "script-command"},
		{"pwsh positional script", "Set-Alias ls Remove-Item", "Bash", "pwsh x.ps1", "script-command"},
		{"powershell command runs a script", "New-PSDrive -Name q -Root y", "Bash", "powershell ./x.ps1", "script-command"},
		{"call operator from the PowerShell tool", "Invoke-History 3", "PowerShell", "& ./x.ps1", "script-command"},
		{"dot source from the PowerShell tool", "iex 'x'", "PowerShell", ". ./x.ps1", "script-command"},
		{"a plain script passes", "Get-ChildItem\nWrite-Output done", "PowerShell", "& ./x.ps1", ""},
		{"a computed name passes inside a script", "& $env:JAVACMD -version", "PowerShell", "& ./x.ps1", ""},
		{"a parse error passes, nothing runs", "Get-Item 'x", "PowerShell", "& ./x.ps1", ""},
		{"a script running itself ends", "& ./x.ps1", "PowerShell", "& ./x.ps1", ""},
		{"secret path keeps its id", "Get-Content ~/.ssh/id_rsa", "PowerShell", "& ./x.ps1", "secret-path"},
		{"stdin stays refused", "", "Bash", "pwsh -File -", "opaque-shell"},
	}
	cases = append(cases, struct{ name, script, tool, command, want string }{"extension in any case", "iex 'x'", "PowerShell", "& ./X.PS1", "script-command"})
	for _, tc := range cases {
		file := "x.ps1"
		if tc.name == "extension in any case" {
			file = "X.PS1"
		}
		dir := writePs1(t, file, tc.script)
		in := Input{Tool: tc.tool, Command: tc.command, Cwd: dir, Home: t.TempDir()}
		got := Decide(in, Policy{})
		if verdictRule(got) != tc.want {
			t.Errorf("%s: verdict %+v, want %q", tc.name, got, tc.want)
			continue
		}
		if tc.want == "script-command" && got.Mode != ModeDeny {
			t.Errorf("%s: mode %v, want deny kept from the locked rule", tc.name, got.Mode)
		}
	}
}

func TestPowerShellScriptRefusalIsSwitchable(t *testing.T) {
	dir := writePs1(t, "x.ps1", "iex 'x'")
	in := Input{Tool: "PowerShell", Command: "& ./x.ps1", Cwd: dir, Home: t.TempDir()}
	if got := Decide(in, Policy{Rules: map[string]string{"script-command": "allow"}}); got != nil {
		t.Errorf("script-command allowed, verdict %+v", got)
	}
	typed := Input{Tool: "PowerShell", Command: "iex 'x'", Cwd: dir, Home: t.TempDir()}
	if got := Decide(typed, Policy{Rules: map[string]string{"script-command": "allow"}}); verdictRule(got) != "powershell-construct" {
		t.Errorf("typed directly, verdict %+v, want powershell-construct", got)
	}
}

func TestPowerShellScriptName(t *testing.T) {
	for name, want := range map[string]bool{"./x.ps1": true, `C:\a\X.PS1`: true, "./x.ps1. ": true, "./x.ps1x": false, "./ps1": false, "./x.psm1": false} {
		if got := powerShellScriptName(name); got != want {
			t.Errorf("powerShellScriptName(%q) = %v, want %v", name, got, want)
		}
	}
}
