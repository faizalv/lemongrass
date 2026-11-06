// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"os/exec"
	"runtime"
	"syscall"
)

// The parent-death signal is tied to the thread that forks, hence the lock.
func runTiedToParent(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	return cmd.Run()
}
