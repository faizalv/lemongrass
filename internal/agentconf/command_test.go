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
