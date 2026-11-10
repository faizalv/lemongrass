// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package gatekeeper

import (
	"encoding/base64"
	"unicode/utf16"

	"github.com/faizalv/lemongrass/internal/vault"
)

// askArgv is the program and arguments a command ask runs as: bash -c for a bash ask, PowerShell without the
// user's profile for a PowerShell one.
func askArgv(shell, command string) ([]string, error) {
	if shell != vault.AskShellPowerShell {
		bash, err := askBash()
		if err != nil {
			return nil, err
		}
		return []string{bash, "-c", command}, nil
	}
	host, err := askPowerShell()
	if err != nil {
		return nil, err
	}
	// The approved text goes in encoded, which the guard refuses from a model: on Windows argv is rebuilt from one
	// command line, and Windows PowerShell mangles a double quote inside -Command text there. The never-execute
	// check and the card read the plain text, so the encoding hides nothing the human did not approve.
	return []string{host, "-NoProfile", "-EncodedCommand", encodePowerShell(command)}, nil
}

// encodePowerShell is the -EncodedCommand form of text: base64 of its UTF-16LE bytes.
func encodePowerShell(text string) string {
	units := utf16.Encode([]rune(text))
	raw := make([]byte, 0, 2*len(units))
	for _, u := range units {
		raw = append(raw, byte(u), byte(u>>8))
	}
	return base64.StdEncoding.EncodeToString(raw)
}
