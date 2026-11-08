// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build windows

package agentconf

import "testing"

func TestHookCommandQuotesWindowsPaths(t *testing.T) {
	for _, c := range []struct{ path, want string }{
		{`C:\Users\ab\.lemongrass\bin\lgrassd.exe`, `C:/Users/ab/.lemongrass/bin/lgrassd.exe hook Stop`},
		{`C:\Users\A B\.lemongrass\bin\lgrassd.exe`, `"C:/Users/A B/.lemongrass/bin/lgrassd.exe" hook Stop`},
		{`C:\Users\RUNNER~1\AppData\lgrassd.exe`, `C:/Users/RUNNER~1/AppData/lgrassd.exe hook Stop`},
	} {
		if got := HookCommand(c.path, "hook", "Stop"); got != c.want {
			t.Errorf("HookCommand(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestPowerShellHookCommandCallsQuotedPaths(t *testing.T) {
	for _, c := range []struct{ path, want string }{
		{`C:\Users\ab\.lemongrass\bin\lgrassd.exe`, `C:/Users/ab/.lemongrass/bin/lgrassd.exe hook Stop`},
		{`C:\Users\A B\.lemongrass\bin\lgrassd.exe`, `& 'C:\Users\A B\.lemongrass\bin\lgrassd.exe' hook Stop`},
	} {
		if got := PowerShellHookCommand(c.path, "hook", "Stop"); got != c.want {
			t.Errorf("PowerShellHookCommand(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestCmdHookCommandEscapesInsteadOfQuoting(t *testing.T) {
	for _, c := range []struct{ path, want string }{
		{`C:\Users\ab\.lemongrass\bin\lgrassd.exe`, `C:/Users/ab/.lemongrass/bin/lgrassd.exe hook Stop`},
		{`C:\Users\A B\.lemongrass\bin\lgrassd.exe`, `C:/Users/A^ B/.lemongrass/bin/lgrassd.exe hook Stop`},
	} {
		if got := CmdHookCommand(c.path, "hook", "Stop"); got != c.want {
			t.Errorf("CmdHookCommand(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
