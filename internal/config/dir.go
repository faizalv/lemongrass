// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Package config holds path helpers for lemongrass's own runtime directory.
package config

import (
	"os"
	"path/filepath"
)

func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".lemongrass")
}
