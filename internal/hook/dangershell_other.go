// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package hook

import (
	"encoding/json"

	"github.com/faizalv/lemongrass/internal/guard"
)

// shellToolCommand returns the shell text the gate judges for a Bash call, the guard tool that reads it, and isShell
// false for any other tool, whose paths are judged instead. A verdict, never set off Windows, denies the call
// without judging it. Off Windows every shell tool runs bash; pwsh -c inside it is unwrapped by the guard.
func shellToolCommand(payload hookEvent) (command, tool string, verdict *guard.Verdict, isShell bool) {
	if payload.ToolName != "Bash" {
		return "", "", nil, false
	}
	var shell shellToolInput
	if err := json.Unmarshal(payload.ToolInput, &shell); err != nil {
		return "", "Bash", nil, true
	}
	return shell.Command, "Bash", nil, true
}
