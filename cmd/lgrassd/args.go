// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import "strings"

// rejoinProgram undoes a program name split at whitespace. cmd.exe, which Antigravity runs its hooks and statusline
// through on Windows, starts lgrassd with its path unquoted on the command line (the caret escapes are consumed), so
// a path holding a space reaches Go as several arguments and the subcommand would be read from the second piece.
// When cmdline starts with exe, compared without case and with either slash, those pieces become one argument again.
// The raw line is used rather than a join of args, which cannot rebuild a run of spaces or a tab. A quoted or
// unrelated argv[0] is returned unchanged.
func rejoinProgram(cmdline, exe string, args []string) []string {
	if len(args) < 2 || exe == "" || len(cmdline) < len(exe) {
		return args
	}
	prefix := cmdline[:len(exe)]
	if !strings.EqualFold(slashed(prefix), slashed(exe)) {
		return args
	}
	if rest := cmdline[len(exe):]; rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return args
	}
	// Go's Windows argument parser splits an unquoted run at spaces and tabs only, so the prefix's words are the
	// arguments it became; any mismatch means the line was not read that way and nothing is changed.
	words := strings.FieldsFunc(prefix, func(r rune) bool { return r == ' ' || r == '\t' })
	if len(words) < 2 || len(words) > len(args) {
		return args
	}
	for i, word := range words {
		if args[i] != word {
			return args
		}
	}
	return append([]string{prefix}, args[len(words):]...)
}

func slashed(path string) string {
	return strings.ReplaceAll(path, `\`, "/")
}
