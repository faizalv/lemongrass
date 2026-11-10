// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import "testing"

func TestNeverExecutePowerShell(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	refused := []string{
		"Remove-Item -Recurse -Force ~",
		"ri -r ~",
		"Remove-Item -Recurse ~/.ssh",
		"& $tool -x",
		"Get-Item 'x",
		"pwsh -NoProfile -Command 'Remove-Item -Recurse ~'",
	}
	for _, command := range refused {
		if reason, ok := NeverExecutePowerShell(command, cwd, home); !ok {
			t.Errorf("%q was not refused", command)
		} else if reason == "" {
			t.Errorf("%q was refused with no reason", command)
		}
	}
	for _, command := range []string{"Get-ChildItem", "Remove-Item -Recurse ./build", "git push origin main", "iex 'Get-Date'", "Invoke-WebRequest https://example.com -OutFile x.zip"} {
		if reason, ok := NeverExecutePowerShell(command, cwd, home); ok {
			t.Errorf("%q was refused: %s", command, reason)
		}
	}
}
