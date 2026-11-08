// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package agentconf

import "testing"

func TestOwnsHookCommand(t *testing.T) {
	for _, c := range []struct {
		command string
		want    bool
	}{
		{"/home/u/.lemongrass/bin/lgrassd hook PreToolUse", true},
		{"'/home/a b/lgrassd' hook PreToolUse", true},
		{`"C:/Users/A B/.lemongrass/bin/lgrassd.exe" hook PreToolUse --vendor codex`, true},
		{"LGRASS_HOOK_VENDOR=codex /x/lgrassd hook PreToolUse", true},
		{"/x/lgrassd hook PreToolUseExtra", false},
		{"/x/lgrassd hook PreToolUse --vendor other", true},
		{"/x/lgrassd hook PreToolUse --vendor agy", true},
		{"/x/lgrassd hook PreToolUse --vendor ", false},
		{"/x/lgrassd hook PreToolUse --vendor agy extra", false},
		{"other-tool PreToolUse", false},
	} {
		if got := OwnsHookCommand(c.command, "PreToolUse"); got != c.want {
			t.Errorf("OwnsHookCommand(%q) = %v, want %v", c.command, got, c.want)
		}
	}
}

func TestCaretEscapeLeavesNoQuote(t *testing.T) {
	for _, c := range []struct{ path, want string }{
		{"C:/Users/ab/bin/lgrassd.exe", "C:/Users/ab/bin/lgrassd.exe"},
		{"C:/Users/A B/bin/lgrassd.exe", "C:/Users/A^ B/bin/lgrassd.exe"},
		{"C:/Users/Tom & Kim (Work)/lgrassd.exe", "C:/Users/Tom^ ^&^ Kim^ ^(Work^)/lgrassd.exe"},
		{"C:/Users/a^b,c;d=e/lgrassd.exe", "C:/Users/a^^b^,c^;d^=e/lgrassd.exe"},
		{"C:/Users/José/lgrassd.exe", "C:/Users/José/lgrassd.exe"},
	} {
		if got := caretEscape(c.path); got != c.want {
			t.Errorf("caretEscape(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestPowerShellCallQuotesForPowerShell(t *testing.T) {
	for _, c := range []struct{ path, want string }{
		{`C:\Users\A B\bin\lgrassd.exe`, `& 'C:\Users\A B\bin\lgrassd.exe'`},
		{`C:\Users\O'Brien $x\lgrassd.exe`, `& 'C:\Users\O''Brien $x\lgrassd.exe'`},
		{"C:\\Users\\O\u2019Neil\\lgrassd.exe", "& 'C:\\Users\\O\u2019\u2019Neil\\lgrassd.exe'"},
	} {
		if got := powerShellCall(c.path); got != c.want {
			t.Errorf("powerShellCall(%q) = %q, want %q", c.path, got, c.want)
		}
	}
	command := joinCommand(powerShellCall(`C:\Users\A B\lgrassd.exe`), []string{"hook", "PreToolUse", VendorFlag, "codex"})
	if !OwnsHookCommand(command, "PreToolUse") {
		t.Errorf("OwnsHookCommand(%q) = false, want true", command)
	}
}

func TestCmdHookCommandStaysOwned(t *testing.T) {
	command := joinCommand(caretEscape("C:/Users/A B/lgrassd.exe"), []string{"hook", "PreToolUse", VendorFlag, "agy"})
	if !OwnsHookCommand(command, "PreToolUse") {
		t.Errorf("OwnsHookCommand(%q) = false, want true", command)
	}
}
