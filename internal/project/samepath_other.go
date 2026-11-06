// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package project

// samePath compares two resolved paths exactly.
func samePath(a, b string) bool {
	return a == b
}
