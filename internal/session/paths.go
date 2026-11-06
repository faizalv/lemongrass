// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Package session needs no PTY access and no Electron dependency.
package session

import (
	"path/filepath"

	"github.com/faizalv/lemongrass/internal/config"
)

func DBPath() string {
	return filepath.Join(config.Dir(), "sessions.db")
}
