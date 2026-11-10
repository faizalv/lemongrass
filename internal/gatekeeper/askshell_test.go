// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package gatekeeper

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/vault"
)

func TestEncodePowerShell(t *testing.T) {
	// pwsh -EncodedCommand reads base64 of UTF-16LE; "a" is 61 00 and U+1F600 a surrogate pair.
	cases := map[string]string{"a": "YQA=", "\U0001F600": "PdgA3g==", "": ""}
	for text, want := range cases {
		if got := encodePowerShell(text); got != want {
			t.Errorf("encodePowerShell(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestAskArgv(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Git Bash may be missing on a test machine; Windows PowerShell never is.
		argv, err := askArgv(vault.AskShellPowerShell, "a")
		if err != nil || len(argv) != 4 || argv[1] != "-NoProfile" || argv[2] != "-EncodedCommand" || argv[3] != "YQA=" {
			t.Fatalf("powershell argv = %q, %v", argv, err)
		}
		if host := strings.ToLower(filepath.Base(argv[0])); host != "pwsh.exe" && host != "powershell.exe" {
			t.Errorf("powershell host = %s", argv[0])
		}
		return
	}
	argv, err := askArgv("", "echo hi")
	if err != nil || len(argv) != 3 || argv[1] != "-c" || argv[2] != "echo hi" {
		t.Fatalf("bash argv = %q, %v", argv, err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := askArgv(vault.AskShellPowerShell, "Get-Date"); err == nil || !strings.Contains(err.Error(), "pwsh is not installed") {
		t.Errorf("powershell argv without pwsh = %v", err)
	}
}

func filePowerShell(t *testing.T, client *Client, command string) vault.Ask {
	t.Helper()
	a, err := client.CreateAsk(AskRequest{Type: vault.AskCommand, TabID: "tab-1", Cwd: t.TempDir(), Command: command, Shell: vault.AskShellPowerShell, Reason: "needed"})
	if err != nil {
		t.Fatalf("CreateAsk: %v", err)
	}
	return a
}

func TestIPCAskPowerShellExecute(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh is not installed")
	}
	client := startAskServer(t)
	a := filePowerShell(t, client, `Write-Output 'say "hi" $x'; exit 4`)
	view, err := client.GetAsk(testRootSecret, a.ID)
	if err != nil || view.Shell != vault.AskShellPowerShell || view.NeverExecute {
		t.Fatalf("GetAsk = %+v, %v", view, err)
	}
	if _, err := client.ExecuteAsk(testRootSecret, a.ID, 0); err != nil {
		t.Fatal(err)
	}
	done := waitAskState(t, client, a.ID, vault.AskAnswered)
	if done.Result == nil || done.Result.ExitCode != 4 || done.Result.OutputTail != `say "hi" $x` {
		t.Fatalf("result = %+v", done.Result)
	}
}

func TestIPCAskPowerShellJudged(t *testing.T) {
	client := startAskServer(t)
	refused := filePowerShell(t, client, "Remove-Item -Recurse -Force ~")
	view, err := client.GetAsk(testRootSecret, refused.ID)
	if err != nil || !view.NeverExecute {
		t.Fatalf("GetAsk = %+v, %v, want never execute", view, err)
	}
	if _, err := client.ExecuteAsk(testRootSecret, refused.ID, 0); err == nil || !strings.Contains(err.Error(), "never executes") {
		t.Fatalf("ExecuteAsk = %v, want a never-execute refusal", err)
	}
	// Read as bash, this text is one harmless word; read as PowerShell it is a construct the guard denies.
	construct := filePowerShell(t, client, "iex 'Get-Date'")
	view, err = client.GetAsk(testRootSecret, construct.ID)
	if err != nil || view.Verdict == nil || view.Verdict.Rule != "powershell-construct" {
		t.Fatalf("verdict = %+v, %v, want powershell-construct", view.Verdict, err)
	}
}
