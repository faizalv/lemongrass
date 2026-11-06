// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package osdeps

import "os/exec"

var lookPath = exec.LookPath
