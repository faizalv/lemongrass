// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHoldsOtherFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if holdsOtherFiles(dir) {
		t.Errorf("an empty folder counts as shared")
	}
	for _, name := range installedBinaries {
		write(name)
		write(name + ".old-1730000000")
	}
	if holdsOtherFiles(dir) {
		t.Errorf("lemongrass's own binaries count as shared")
	}
	write("claude.exe")
	if !holdsOtherFiles(dir) {
		t.Errorf("a folder holding claude.exe does not count as shared")
	}
	if holdsOtherFiles(filepath.Join(dir, "missing")) {
		t.Errorf("a missing folder counts as shared")
	}
}
