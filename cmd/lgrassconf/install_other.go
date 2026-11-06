// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !linux && !darwin && !windows

package main

import "fmt"

func installService(home string) error {
	_ = home
	return fmt.Errorf("lgrassconf install is only supported on Linux (systemd), macOS (LaunchAgent) and Windows (Task Scheduler)")
}

func uninstallService(home string) error {
	_ = home
	return fmt.Errorf("lgrassconf uninstall is only supported on Linux (systemd), macOS (LaunchAgent) and Windows (Task Scheduler)")
}
