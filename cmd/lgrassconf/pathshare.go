// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"os"
	"strings"
)

// holdsOtherFiles reports whether dir holds anything besides lemongrass's own binaries and the copies the app renamed
// aside while they ran. ~/.local/bin is shared, Claude Code's install script puts claude there and relies on it being
// on PATH, so uninstall keeps the PATH entry it added while anything else lives in the folder. An unreadable folder
// counts as shared, since dropping the entry is the step that can break another tool.
func holdsOtherFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return !os.IsNotExist(err)
	}
	for _, e := range entries {
		if !ownBinary(e.Name()) {
			return true
		}
	}
	return false
}

func ownBinary(name string) bool {
	for _, own := range installedBinaries {
		if strings.EqualFold(name, own) || strings.HasPrefix(strings.ToLower(name), strings.ToLower(own)+".old-") {
			return true
		}
	}
	return false
}
