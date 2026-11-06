// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package ipc

// SameExecutable reports whether two binary paths name the same file.
func SameExecutable(a, b string) bool {
	return a == b
}
