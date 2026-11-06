// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Package gitbash finds Git for Windows' bash, the shell lemongrass runs a user's command line through on Windows,
// as Claude Code does. It never returns System32\bash.exe, which is WSL and sees a different filesystem.
package gitbash
