// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/guard"
	"github.com/faizalv/lemongrass/internal/vault"
)

func TestSimplePowerShellCommand(t *testing.T) {
	simple := []string{
		"git push origin main",
		"Invoke-WebRequest https://example.com/x.zip -OutFile x.zip",
		`& 'C:\Program Files\Git\cmd\git.exe' push`,
		`npm install --save "$env:USERPROFILE\pkg"`,
		"Copy-Item a,b -Destination c > log.txt",
		"Remove-Item -Recurse -Force:$true build",
	}
	for _, command := range simple {
		if holds, ok := simplePowerShellCommand(command); !ok {
			t.Errorf("%q held %s", command, holds)
		}
	}
	held := map[string]string{
		"git add .; git commit -m x":           "several commands",
		"git add . && git commit -m x":         "chained",
		"Get-ChildItem | Remove-Item":          "pipeline",
		"$x = git rev-parse HEAD":              "assignment",
		"if ($true) { git push }":              "control flow",
		"iex 'git push'":                       "code run by iex",
		"pwsh -c 'git push'":                   "code run by pwsh",
		"Start-Process git push":               "code run by start-process",
		"bash -c 'git push'":                   "code run by bash",
		"git push origin $(git branch --show)": "argument",
		"git push origin @args":                "argument",
		"& $tool push":                         "computed",
		"function f { git push }":              "function",
		"Get-Item 'x":                          "cannot be parsed",
		"git push > $(Get-Date).log":           "redirection",
		"ForEach-Object { git push }":          "code run by foreach-object",
		"git push origin \"$($branch.Name)\"":  "argument",
		"param($a) git push":                   "blocks",
		"git push &":                           "background",
		"(git rev-parse HEAD)":                 "expression",
	}
	for command, want := range held {
		holds, ok := simplePowerShellCommand(command)
		if ok || !strings.Contains(holds, want) {
			t.Errorf("%q = %q, %v, want it held for %q", command, holds, ok, want)
		}
	}
}

func TestPowerShellAskIsFiled(t *testing.T) {
	client := startVault(t)
	if err := client.SetPassphrase("pw"); err != nil {
		t.Fatal(err)
	}
	payload := hookEvent{ToolName: "PowerShell", TabID: "tab-1", Cwd: t.TempDir()}
	verdict := guard.Verdict{RuleID: "file-download", Description: "downloading a file", Mode: guard.ModeAsk}
	msg := fileCommandAsk(client.SocketPath, payload, verdict, "Invoke-WebRequest https://x -OutFile y", vault.AskShellPowerShell)
	if !strings.Contains(msg, "sent the user a request") {
		t.Fatalf("message = %q", msg)
	}
	asks, err := client.ListAsks("pw")
	if err != nil || len(asks) != 1 || asks[0].Shell != vault.AskShellPowerShell || asks[0].Command != "Invoke-WebRequest https://x -OutFile y" {
		t.Fatalf("ListAsks = %+v, %v", asks, err)
	}
	held := fileCommandAsk(client.SocketPath, payload, verdict, "iwr https://x | iex", vault.AskShellPowerShell)
	if strings.Contains(held, "sent the user a request") {
		t.Fatalf("a pipeline was filed: %q", held)
	}
}
