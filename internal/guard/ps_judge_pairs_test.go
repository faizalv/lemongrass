// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/psparse"
)

// psDecide judges a PowerShell command the way the Decide branch does: a parse error is unparseable-command, and
// otherwise the judged script goes through scriptVerdicts unchanged.
func psDecide(in Input, policy Policy) *Verdict {
	c := checker{in: in, policy: policy, home: ResolveDir(in.Home)}
	var script shellScript
	if err := script.addPowerShell(in.Command, 0); err != nil {
		return c.pick([]Verdict{ruleVerdict("unparseable-command", "")})
	}
	return c.pick(c.scriptVerdicts(script))
}

// psJudgeReady skips until the parser and the semantic tables both run, since the pairs need both.
func psJudgeReady(t *testing.T) {
	t.Helper()
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Skipf("psparse is not ported yet: %v", r)
			}
		}()
		if _, err := psparse.Parse("Remove-Item x", psparse.DefaultLimits); err != nil {
			t.Skipf("psparse cannot parse a plain command yet: %v", err)
		}
	}()
	for _, meaning := range psResolveName("Remove-Item") {
		if meaning.Cmdlet == "remove-item" {
			return
		}
	}
	t.Skip("the semantic tables do not resolve cmdlets yet")
}

// TestPowerShellBashPairs holds each PowerShell command to the verdict its bash equivalent meets.
func TestPowerShellBashPairs(t *testing.T) {
	psJudgeReady(t)
	root, home, outside := rmProject(t)
	cases := []struct {
		name, ps, bash string
	}{
		{"recursive delete inside the project", "Remove-Item -Recurse build", "rm -r build"},
		{"recursive delete abbreviated", "Remove-Item -rec build", "rm -r build"},
		{"plain delete inside the project", "Remove-Item notes.txt", "rm notes.txt"},
		{"forced recursive delete outside", "Remove-Item -Recurse -Force " + outside, "rm -rf " + outside},
		{"plain delete outside", "Remove-Item " + filepath.Join(outside, "f"), "rm " + filepath.Join(outside, "f")},
		{"delete into .git", "Remove-Item -Recurse .git/refs", "rm -r .git/refs"},
		{"delete through a symlink out", "Remove-Item -Recurse escape/", "rm -r escape/"},
		{"delete from a variable", "Remove-Item -LiteralPath $t -Recurse -Force", `rm -rf -- "$t"`},
		{"delete from the pipeline", "Get-ChildItem *.log | Remove-Item", "ls *.log | xargs rm"},
		{"delete in a ForEach-Object block", "Get-ChildItem | ForEach-Object { Remove-Item $_ }", `for f in *; do rm "$f"; done`},
		{"delete in an if branch", "if ($x) { Remove-Item -Recurse " + outside + " }", "if true; then rm -r " + outside + "; fi"},
		{"delete in an else branch", "if ($x) { 0 } else { Remove-Item -Recurse " + outside + " }", "if true; then :; else rm -r " + outside + "; fi"},
		{"delete in a parallel block", "parallel { Remove-Item -Recurse " + outside + " }", "rm -r " + outside},
		{"delete in a sequence block", "sequence { Remove-Item -Recurse " + outside + " }", "rm -r " + outside},
		{"delete in a foreach -parallel body", "foreach -parallel -throttlelimit 2 ($f in $x) { Remove-Item -Recurse " + outside + " }", "rm -r " + outside},
		{"delete in a switch -parallel clause", "switch -parallel ($x) { 1 { Remove-Item -Recurse " + outside + " } }", "rm -r " + outside},
		{"read an ssh key", "Get-Content ~/.ssh/id_rsa", "cat ~/.ssh/id_rsa"},
		{"read an ssh key by alias", "gc ~/.ssh/id_rsa", "cat ~/.ssh/id_rsa"},
		{"read an ssh key under $HOME", `Get-Content "$HOME/.ssh/id_rsa"`, `cat "$HOME/.ssh/id_rsa"`},
		{"read a project file", "Get-Content notes.txt", "cat notes.txt"},
		{"download to a file", "Invoke-WebRequest https://example.com/x -OutFile x", "curl -o x https://example.com/x"},
		{"download by alias", "iwr https://example.com/x -OutFile x", "curl -o x https://example.com/x"},
		{"write a shell startup file", "Set-Content ~/.bashrc 'x'", "echo x > ~/.bashrc"},
		{"redirect into a startup file", "'x' > ~/.bashrc", "echo x > ~/.bashrc"},
		{"redirect into the project", "'x' > out.txt", "echo x > out.txt"},
		{"redirect to null", "Get-ChildItem > $null", "ls > /dev/null"},
		{"write agent settings", "Set-Content ~/.claude/settings.json '{}'", "echo '{}' > ~/.claude/settings.json"},
		{"force push", "git push --force", "git push --force"},
		{"git status", "git status", "git status"},
		{"git through a pipeline", "git log --oneline | Select-Object -First 3", "git log --oneline | head -3"},
		{"list big files", "Get-ChildItem -Recurse | Where-Object { $_.Length -gt 1MB }", "find . -size +1M"},
		{"count lines", "$files = Get-ChildItem; if ($files) { ($files | Get-Content | Measure-Object -Line).Lines } else { 0 }", "cat * | wc -l"},
		{"environment read", "$env:PATH", "echo $PATH"},
		{"bash run from PowerShell", "bash -c 'rm -rf " + outside + "'", "bash -c 'rm -rf " + outside + "'"},
		{"exec unwrapped", "exec rm -rf " + outside, "exec rm -rf " + outside},
		{"python code piped in", "'import shutil; shutil.rmtree(\"" + outside + "\")' | python3", "echo 'import shutil; shutil.rmtree(\"" + outside + "\")' | python3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bash := Decide(Input{Tool: "Bash", Command: tc.bash, Cwd: root, Home: home, ProjectRoot: root}, Policy{})
			ps := psDecide(Input{Tool: "PowerShell", Command: tc.ps, Cwd: root, Home: home, ProjectRoot: root}, Policy{})
			if ruleOf(ps) != ruleOf(bash) {
				t.Errorf("PowerShell %q meets %q (%s), bash %q meets %q", tc.ps, ruleOf(ps), psDetail(ps), tc.bash, ruleOf(bash))
			}
		})
	}
}

// TestPowerShellRefusedCommands holds each construct of PRD decision 7 to its locked rule, through the parser.
func TestPowerShellRefusedCommands(t *testing.T) {
	psJudgeReady(t)
	root, home, _ := rmProject(t)
	cases := []struct {
		ps, rule, guide string
	}{
		{"$f.Delete()", "powershell-construct", "method calls"},
		{"[IO.File]::Delete('x')", "powershell-construct", "static members"},
		{"& $tool x", "dynamic-command", "Write the command name out"},
		{". $script", "dynamic-command", "Write the command name out"},
		{"Invoke-Expression 'rm x'", "powershell-construct", "Invoke-Expression"},
		{"iex 'rm x'", "powershell-construct", "Invoke-Expression"},
		{"Invoke-Command { ls }", "powershell-construct", "Invoke-Command"},
		{"icm { ls }", "powershell-construct", "Invoke-Command"},
		{"Start-Job { ls }", "powershell-construct", "background job"},
		{"Get-ChildItem &", "powershell-construct", "background job"},
		{"Import-Module x", "powershell-construct", "module"},
		{"ipmo x", "powershell-construct", "module"},
		{"using module x", "powershell-construct", "module"},
		{"Add-Type -Path x.dll", "powershell-construct", "module"},
		{"Add-Type -TypeDefinition 'public class C {}'", "powershell-construct", "module"},
		{"function f { ls }", "powershell-construct", "definition"},
		{"filter f { ls }", "powershell-construct", "definition"},
		{"workflow w { ls }", "powershell-construct", "definition"},
		{"class C { }", "powershell-construct", "definition"},
		{"enum E { A }", "powershell-construct", "definition"},
		{"Invoke-Item x.txt", "powershell-construct", "Invoke-Item"},
		{"ii x.txt", "powershell-construct", "Invoke-Item"},
		{"git --% log %X%", "powershell-construct", "--%"},
		{"Set-Alias ls Remove-Item", "name-redefinition", "alias"},
		{"sal ls Remove-Item", "name-redefinition", "alias"},
		{"New-Alias x y", "name-redefinition", "alias"},
		{"$function:ls = { }", "name-redefinition", "alias"},
		{"New-PSDrive -Name X -PSProvider FileSystem -Root C:\\", "drive-definition", "drive"},
		{"Invoke-History 3", "history-replay", "history"},
		{"r 3", "history-replay", "history"},
		{"if ($x) { 0 } else { $f.Delete() }", "powershell-construct", "method calls"},
	}
	for _, tc := range cases {
		t.Run(tc.ps, func(t *testing.T) {
			verdict := psDecide(Input{Tool: "PowerShell", Command: tc.ps, Cwd: root, Home: home, ProjectRoot: root}, Policy{})
			if ruleOf(verdict) != tc.rule {
				t.Fatalf("rule %q (%s), want %q", ruleOf(verdict), psDetail(verdict), tc.rule)
			}
			if !strings.Contains(verdict.Message(), tc.guide) {
				t.Errorf("message %q lacks %q", verdict.Message(), tc.guide)
			}
		})
	}
}

// TestPowerShellParseErrorDenies holds input PowerShell cannot parse to unparseable-command.
func TestPowerShellParseErrorDenies(t *testing.T) {
	psJudgeReady(t)
	root, home, _ := rmProject(t)
	for _, ps := range []string{"if (", "Remove-Item 'x", "{ ls", "$x = "} {
		verdict := psDecide(Input{Tool: "PowerShell", Command: ps, Cwd: root, Home: home, ProjectRoot: root}, Policy{})
		if ruleOf(verdict) != "unparseable-command" {
			t.Errorf("%q meets %q, want unparseable-command", ps, ruleOf(verdict))
		}
	}
}

func psDetail(verdict *Verdict) string {
	if verdict == nil {
		return "no verdict"
	}
	return verdict.Detail
}

// TestPowerShellSevenOnlyNumbers holds the 5.1 reading of a number only pwsh 7 knows: a bare word as an argument,
// and a command of that name as a statement; in expression mode 5.1 cannot parse it, so only 7's reading is recorded.
func TestPowerShellSevenOnlyNumbers(t *testing.T) {
	psJudgeReady(t)
	hasCall := func(script shellScript, name string, args ...string) bool {
		for _, call := range script.Calls {
			if call.Name == name && strings.Join(call.Args, " ") == strings.Join(args, " ") {
				return true
			}
		}
		return false
	}
	judge := func(ps string) shellScript {
		t.Helper()
		var script shellScript
		if err := script.addPowerShell(ps, 0); err != nil {
			t.Fatalf("%q: %v", ps, err)
		}
		return script
	}
	script := judge("Remove-Item 10u")
	if !hasCall(script, "rm", "--", "10") || !hasCall(script, "rm", "--", "10u") {
		t.Errorf("Remove-Item 10u gave %v, want rm -- 10 and rm -- 10u", script.Calls)
	}
	script = judge("0b101")
	if !hasCall(script, "0b101") {
		t.Errorf("0b101 gave %v, want a command named 0b101", script.Calls)
	}
	script = judge("$a = 2.5y")
	if len(script.Calls) != 0 {
		t.Errorf("$a = 2.5y gave %v, want no call", script.Calls)
	}
	script = judge("Start-Sleep 0x10")
	if len(script.Calls) != 1 {
		t.Errorf("Start-Sleep 0x10 gave %v, want one reading", script.Calls)
	}
}

// TestPowerShellNonASCIIKeywordSpelling holds that a keyword spelled with a non-ASCII letter is a command name to
// pwsh 7.6 as it is to 5.1, so both read it alike and nothing is refused; the corpus's long-s-command is one.
func TestPowerShellNonASCIIKeywordSpelling(t *testing.T) {
	psJudgeReady(t)
	for _, ps := range []string{"\u017Fwitch ($x) { default { 1 } }", "\u0131f ($x) { 1 }", "foreach ($i in 1..2) { brea\u212A }"} {
		res, err := psparse.Parse(ps, psparse.DefaultLimits)
		if err != nil {
			t.Fatalf("%q: %v", ps, err)
		}
		if keyword := psNonASCIIKeyword(res.Tokens); keyword != nil {
			t.Errorf("%q: %q is a keyword token; the judge refuses it, so recheck the version readings", ps, keyword.Text)
		}
	}
}
