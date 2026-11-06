// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build linux

package gatekeeper

import "syscall"

// A command must not outlive the daemon that holds its ask, since nothing would be left to report its result.
func setAskParentDeath(attr *syscall.SysProcAttr) {
	attr.Pdeathsig = syscall.SIGKILL
}
