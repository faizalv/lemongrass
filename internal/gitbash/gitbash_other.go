// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package gitbash

import "errors"

var errNotWindows = errors.New("gitbash: Git Bash exists only on Windows")

// Root is the Git for Windows install root, the directory holding usr, etc and bin.
func Root() (string, error) {
	return "", errNotWindows
}

// Bash is the full path of Git Bash's bash.exe.
func Bash() (string, error) {
	return "", errNotWindows
}
