// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !linux

package hook

// Ending the job needs the process tree read from /proc; other systems keep the prompt block and tool deny.
func startShellTabEnd() {}

func runShellTabEnd([]string) int {
	return 1
}
