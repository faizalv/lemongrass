// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"embed"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/faizalv/lemongrass/internal/bibliothek"
)

//go:embed orientation
var orientationFS embed.FS

func (k *keeper) orientationDir() string {
	return filepath.Join(k.home, ".lemongrass", "orientation")
}

const bibliothekHeading = "\n# The bibliothek convention\n\n"

// Each orientation source is installed once per vendor; Codex's and Antigravity's copies carry their prefix.
var orientationVendors = []struct{ name, prefix string }{{"claude", ""}, {"codex", "codex-"}, {"agy", "agy-"}}

const vendorBlockEnd = "<!-- end -->"

// forVendor keeps the lines outside any vendor block and the lines of this vendor's blocks. A block is a line
// such as "<!-- claude -->" or "<!-- codex agy -->", naming one or more vendors, through the next "<!-- end -->" line.
func forVendor(src []byte, vendor string) []byte {
	lines := strings.SplitAfter(string(src), "\n")
	var out strings.Builder
	inBlock, keep, dropped := false, true, false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case !inBlock && strings.HasPrefix(trimmed, "<!-- ") && strings.HasSuffix(trimmed, " -->") && trimmed != vendorBlockEnd:
			inBlock, keep = true, slices.Contains(strings.Fields(strings.TrimSuffix(strings.TrimPrefix(trimmed, "<!--"), "-->")), vendor)
			continue
		case inBlock && trimmed == vendorBlockEnd:
			// Only a block that stood as its own paragraph leaves a doubled gap behind.
			inBlock, dropped = false, !keep && (out.Len() == 0 || strings.HasSuffix(out.String(), "\n\n"))
			continue
		case inBlock && !keep:
			continue
		}
		if dropped && trimmed == "" {
			dropped = false
			continue
		}
		dropped = false
		out.WriteString(line)
	}
	return []byte(out.String())
}

// Each installed orientation is followed by bibliothek's text, so a tab never loads the skill.
func (k *keeper) loadOrientation() []skillFile {
	entries, err := fs.ReadDir(orientationFS, "orientation")
	if err != nil {
		panic(err)
	}
	convention, err := bibliothek.Prompt()
	if err != nil {
		panic(err)
	}
	var files []skillFile
	for _, e := range entries {
		data, err := orientationFS.ReadFile("orientation/" + e.Name())
		if err != nil {
			panic(err)
		}
		for _, v := range orientationVendors {
			content := append(append(forVendor(data, v.name), bibliothekHeading...), convention...)
			path := filepath.Join(k.orientationDir(), v.prefix+e.Name())
			files = append(files, skillFile{root: k.orientationDir(), path: path, content: content})
		}
	}
	return files
}

func (k *keeper) managedFiles() []skillFile {
	files := make([]skillFile, 0, len(k.orientation)+len(k.plugin))
	return append(append(files, k.orientation...), k.plugin...)
}
