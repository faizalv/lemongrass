// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build windows

package ipc

import (
	"path/filepath"
	"strings"
)

// SameExecutable reports whether two binary paths name the same file; NTFS paths compare without case.
func SameExecutable(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
