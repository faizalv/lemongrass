// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build windows

package main

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// programArgs returns os.Args with a program path split at whitespace rejoined, see rejoinProgram.
func programArgs() []string {
	exe, err := os.Executable()
	if err != nil {
		return os.Args
	}
	return rejoinProgram(windows.UTF16PtrToString(syscall.GetCommandLine()), exe, os.Args)
}
