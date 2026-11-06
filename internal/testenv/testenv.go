// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Package testenv holds helpers shared by tests across packages.
package testenv

import "testing"

// Home points the user's home directory at a fresh temp dir for the rest of the test and returns it. HOME alone is
// not enough: on Windows os.UserHomeDir reads USERPROFILE and ignores HOME, so tests would share the runner's real
// ~/.lemongrass.
func Home(t testing.TB) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}
