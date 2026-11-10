// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package vault

import "testing"

func TestAskShell(t *testing.T) {
	s, _, _ := newTestAskStore()
	ps := commandAsk("t1", "Get-ChildItem")
	ps.Shell = AskShellPowerShell
	a, err := s.Create(ps)
	if err != nil || a.Shell != AskShellPowerShell {
		t.Fatalf("powershell ask = %+v, %v", a, err)
	}
	bash := commandAsk("t1", "ls")
	bash.Shell = AskShellBash
	if a, err := s.Create(bash); err != nil || a.Shell != "" {
		t.Fatalf("bash ask = %+v, %v, want the shell stored empty", a, err)
	}
	bad := commandAsk("t1", "ls")
	bad.Shell = "cmd"
	if _, err := s.Create(bad); err == nil {
		t.Error("an unknown shell was accepted")
	}
	db := AskInput{Type: AskDB, TabID: "t1", Cwd: askCwd, Tables: []string{"a"}, Operations: []string{"select"}, Reason: "r", Shell: AskShellPowerShell}
	if _, err := s.Create(db); err == nil {
		t.Error("a db ask with a shell was accepted")
	}
}

// The hook folds a repeated refusal into the open ask only for the same command in the same shell.
func TestAskHookDedupeByShell(t *testing.T) {
	s, _, _ := newTestAskStore()
	hook := AskInput{Type: AskCommand, TabID: "t1", Cwd: askCwd, Command: "git push", Rule: "git-push"}
	first, err := s.Create(hook)
	if err != nil {
		t.Fatal(err)
	}
	hook.Shell = AskShellPowerShell
	ps, err := s.Create(hook)
	if err != nil || ps.ID == first.ID {
		t.Fatalf("a powershell ask with the same text = %v, %v, want a new ask", ps.ID, err)
	}
	again, err := s.Create(hook)
	if err != nil || again.ID != ps.ID {
		t.Fatalf("a repeated powershell hook ask = %v, %v, want %s", again.ID, err, ps.ID)
	}
	hook.Shell = AskShellBash
	if bash, _ := s.Create(hook); bash.ID != first.ID {
		t.Errorf("an explicit bash ask = %s, want the first ask %s", bash.ID, first.ID)
	}
}
