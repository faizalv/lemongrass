// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package main

func replayShell() (string, error) {
	return "bash", nil
}
