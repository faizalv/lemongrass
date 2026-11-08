// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"strings"
	"testing"
)

func verdictRule(v *Verdict) string {
	if v == nil {
		return ""
	}
	return v.RuleID
}

func TestPwshCommandLine(t *testing.T) {
	cases := map[string]string{
		"pwsh -NoProfile -Command Get-Date":             "",
		"pwsh -nop -noni -c Get-ChildItem":              "",
		"pwsh -ExecutionPolicy Bypass -Command 'iex x'": "powershell-construct",
		"powershell Get-Date":                           "",
		"powershell -NoProfile iex x":                   "powershell-construct",
		"pwsh -c 'Set-Alias ls Remove-Item'":            "name-redefinition",
		"pwsh -EncodedCommand AAAA":                     "powershell-construct",
		"pwsh -ec AAAA":                                 "powershell-construct",
		"pwsh -File x.ps1":                              "opaque-shell",
		"pwsh x.ps1":                                    "opaque-shell",
		"pwsh":                                          "opaque-shell",
		"pwsh -NoProfile":                               "opaque-shell",
		"pwsh -c -":                                     "opaque-shell",
		"pwsh -in xml -c Get-Date":                      "opaque-shell",
		"pwsh /c Get-Date":                              "opaque-shell",
		"pwsh -Bogus -c Get-Date":                       "opaque-shell",
		`pwsh -c "$X"`:                                  "dynamic-command",
		"pwsh --version":                                "",
		"pwsh -v":                                       "",
		"pwsh -?":                                       "",
		"powershell /?":                                 "",
		"powershell -Version":                           "",
		"pwsh -c 'Get-Item '\\''x'":                     "unparseable-command",
		`bash -c "pwsh -c 'iex x'"`:                     "powershell-construct",
		"pwsh -c \"bash -c 'iex x'\"":                   "",
		"pwsh -c \"bash -c 'git push --force'\"":        "git-push-force",
		"/mnt/c/Windows/System32/bash.exe -c ls":        "opaque-shell",
	}
	for command, want := range cases {
		if got := verdictRule(Decide(bashInput(t, command), Policy{})); got != want {
			t.Errorf("%s = %q, want %q", command, got, want)
		}
	}
}

func TestPwshWorkingDirectoryIsCd(t *testing.T) {
	script, err := parseShell("pwsh -wd sub -c Get-Date")
	if err != nil {
		t.Fatal(err)
	}
	if len(script.Calls) < 2 || script.Calls[1].Name != "cd" || len(script.Calls[1].Args) != 1 || script.Calls[1].Args[0] != "sub" {
		t.Errorf("calls = %+v, want pwsh then cd sub", script.Calls)
	}
}

func TestPwshParamAbbreviations(t *testing.T) {
	cases := map[string]string{"-c": "command", "-com": "command", "-Command": "command", "-cwa": "commandwithargs", "-e": "encodedcommand", "-ex": "executionpolicy", "-nop": "noprofile", "–c": "command", "--file": "file"}
	for arg, want := range cases {
		p, ok, ambiguous, unknown := pwshParamOf(arg)
		if !ok || ambiguous || unknown || p.name != want {
			t.Errorf("%s = %+v ok=%v ambiguous=%v unknown=%v, want %s", arg, p, ok, ambiguous, unknown, want)
		}
	}
	for _, arg := range []string{"-in", "/command"} {
		if _, _, ambiguous, _ := pwshParamOf(arg); !ambiguous {
			t.Errorf("%s is not ambiguous", arg)
		}
	}
	if _, ok, _, unknown := pwshParamOf("-no"); ok || !unknown {
		t.Error("-no was read")
	}
	if _, ok, ambiguous, _ := pwshParamOf("/tmp/x.ps1"); ok || ambiguous {
		t.Error("a path was read as a parameter")
	}
}

func TestDecidePowerShell(t *testing.T) {
	run := func(command string) *Verdict {
		return Decide(Input{Tool: "PowerShell", Command: command, Cwd: t.TempDir(), Home: t.TempDir()}, Policy{})
	}
	for command, want := range map[string]string{
		"Get-ChildItem":                   "",
		"":                                "",
		"iex 'x'":                         "powershell-construct",
		"New-PSDrive -Name x -Root y":     "drive-definition",
		"Enter-PSSession -ComputerName h": "remote-shell",
		"git push --force":                "git-push-force",
	} {
		if got := verdictRule(run(command)); got != want {
			t.Errorf("%q = %q, want %q", command, got, want)
		}
	}
	v := run("Get-Item 'x")
	if v == nil || v.RuleID != "unparseable-command" || !strings.HasPrefix(v.Detail, "PowerShell reports ") {
		t.Errorf("parse error verdict = %+v", v)
	}
	if v := run("Remove-Item -Recurse -Force ~"); v == nil || v.Mode != ModeDeny {
		t.Errorf("Remove-Item of the home = %+v, want a deny", v)
	}
}
