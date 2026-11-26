// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"os"

	"github.com/faizalv/lemongrass/internal/guard"
	"github.com/faizalv/lemongrass/internal/session"
)

// tempWriteDeny denies a file tool writing into the system temp dir in a bibliothek project, since a reboot or power cut wipes it and a file the model writes itself is costly to lose. Shell writes stay open so what a command regenerates lives there instead of wearing the drive. A write inside any of the call's project roots never counts, so a project living under a temp dir keeps working.
func tempWriteDeny(payload hookEvent, roots ...string) string {
	if !isMemoryWriteTool(payload.ToolName) {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	temp := guard.TempRoots()
	for _, path := range memoryToolPaths(payload) {
		if inTempOutside(payload, home, path, roots, temp) {
			return session.FormatTempWriteDeny(path)
		}
	}
	return ""
}

// inTempOutside reports whether path lands in a temp dir and inside none of roots.
func inTempOutside(payload hookEvent, home, path string, roots, temp []string) bool {
	if len(roots) == 0 {
		roots = []string{""}
	}
	for _, root := range roots {
		in := guard.Input{Tool: payload.ToolName, Cwd: payload.Cwd, Home: home, ProjectRoot: root, Paths: []string{path}, Writes: true}
		if guard.TempWrite(in, temp) == "" {
			return false
		}
	}
	return true
}
