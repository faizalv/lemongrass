// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package main

import "os"

// programArgs is os.Args as is: a POSIX exec passes argv as a list, so nothing splits the program path.
func programArgs() []string {
	return os.Args
}
