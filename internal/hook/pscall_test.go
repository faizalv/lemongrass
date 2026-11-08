// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import "testing"

func TestPowerShellPosixScript(t *testing.T) {
	const bash = `& 'C:\Program Files\Git\bin\bash.exe'`
	cases := []struct {
		name    string
		line    string
		script  string
		refused bool
		matched bool
	}{
		{"full path with -c", bash + ` -c 'ls -la'`, "ls -la", false, true},
		{"login flag", bash + ` -lc 'git status'`, "git status", false, true},
		{"separate flags", bash + ` -l -c 'git status'`, "git status", false, true},
		{"no space after the operator", `&'C:\Program Files\Git\bin\bash.exe' -c 'ls -la'`, "ls -la", false, true},
		{"doubled quote is one quote", bash + ` -c 'echo ''a b'''`, "echo 'a b'", false, true},
		{"bare path", `& C:\Git\bin\bash.exe -c 'ls -la'`, "ls -la", false, true},
		{"one safe word", bash + ` -c 'pwd'`, "pwd", false, true},
		{"double quote inside", bash + ` -c 'git commit -m "wip"'`, "", true, true},
		{"typographic double quote inside", bash + " -c 'echo \u201cx\u201d now'", "", true, true},
		{"trailing backslash", bash + ` -c 'ls C:\x\'`, "", true, true},
		{"empty script", bash + ` -c ''`, "", true, true},
		{"one word with a glob", bash + ` -c 'rm*'`, "", true, true},
		{"statement after the call", bash + ` -c 'ls -la'; Remove-Item -Recurse x`, "", false, false},
		{"second statement on a new line", bash + " -c 'ls -la'\nRemove-Item x", "", false, false},
		{"argument after the script", bash + ` -c 'ls -la' x`, "", false, false},
		{"quoted word runs into the next", bash + ` -c 'ls -la'x`, "", false, false},
		{"unterminated quote", bash + ` -c 'ls -la`, "", false, false},
		{"option that takes an argument", bash + ` -o -c 'ls -la'`, "", false, false},
		{"long option", bash + ` --norc -c 'ls -la'`, "", false, false},
		{"no -c", bash + ` -l 'ls -la'`, "", false, false},
		{"script not quoted", bash + ` -c ls`, "", false, false},
		{"wsl bash", `& 'C:\Windows\System32\bash.exe' -c 'ls -la'`, "", false, false},
		{"not a posix shell", `& 'C:\Tools\pwsh.exe' -c 'ls -la'`, "", false, false},
		{"typographic single quote", "& \u2018C:\\Git\\bin\\bash.exe\u2019 -c 'ls -la'", "", false, false},
		{"variable in a bare word", `& $env:GIT\bash.exe -c 'ls -la'`, "", false, false},
		{"no call operator", `'C:\Git\bin\bash.exe' -c 'ls -la'`, "", false, false},
	}
	for _, tc := range cases {
		script, refusal, matched := powerShellPosixScript(tc.line)
		if script != tc.script || (refusal != "") != tc.refused || matched != tc.matched {
			t.Errorf("%s: got %q, refusal %q, matched %v", tc.name, script, refusal, matched)
		}
	}
}

func TestPSSingleQuote(t *testing.T) {
	if got := psSingleQuote(`C:\Users\O'Brien\bash.exe`); got != `'C:\Users\O''Brien\bash.exe'` {
		t.Errorf("psSingleQuote() = %s", got)
	}
}
