// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestRejoinProgram(t *testing.T) {
	const exe = `C:\Users\Pavilion Aero\.lemongrass\bin\lgrassd.exe`
	tests := []struct {
		name    string
		cmdline string
		exe     string
		want    []string
	}{
		{
			name:    "cmd passes the path unquoted with forward slashes",
			cmdline: "C:/Users/Pavilion Aero/.lemongrass/bin/lgrassd.exe hook PreToolUse --vendor agy",
			exe:     exe,
			want:    []string{"C:/Users/Pavilion Aero/.lemongrass/bin/lgrassd.exe", "hook", "PreToolUse", "--vendor", "agy"},
		},
		{
			name:    "case differs",
			cmdline: `c:\users\pavilion aero\.lemongrass\bin\LGRASSD.EXE statusline --vendor agy`,
			exe:     exe,
			want:    []string{`c:\users\pavilion aero\.lemongrass\bin\LGRASSD.EXE`, "statusline", "--vendor", "agy"},
		},
		{
			name:    "a run of spaces and a tab",
			cmdline: "C:/Users/A  B\tC/lgrassd.exe version",
			exe:     `C:\Users\A  B` + "\t" + `C\lgrassd.exe`,
			want:    []string{"C:/Users/A  B\tC/lgrassd.exe", "version"},
		},
		{
			name:    "several spaces in the path",
			cmdline: "C:/Users/A B C/lgrassd.exe version",
			exe:     `C:\Users\A B C\lgrassd.exe`,
			want:    []string{"C:/Users/A B C/lgrassd.exe", "version"},
		},
		{
			name:    "no subcommand",
			cmdline: "C:/Users/Pavilion Aero/.lemongrass/bin/lgrassd.exe",
			exe:     exe,
			want:    []string{"C:/Users/Pavilion Aero/.lemongrass/bin/lgrassd.exe"},
		},
		{
			name:    "quoted path is already one argument",
			cmdline: `"C:\Users\Pavilion Aero\.lemongrass\bin\lgrassd.exe" hook PreToolUse`,
			exe:     exe,
		},
		{
			name:    "path without a space",
			cmdline: `C:\Users\alex\.lemongrass\bin\lgrassd.exe hook PreToolUse`,
			exe:     `C:\Users\alex\.lemongrass\bin\lgrassd.exe`,
		},
		{
			name:    "bare program name",
			cmdline: "lgrassd hook PreToolUse",
			exe:     exe,
		},
		{
			name:    "prefix runs into a longer name",
			cmdline: "C:/Users/Pavilion Aero/.lemongrass/bin/lgrassd.exe2 hook",
			exe:     exe,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := strings.FieldsFunc(tt.cmdline, func(r rune) bool { return r == ' ' || r == '\t' })
			if strings.HasPrefix(tt.cmdline, `"`) {
				args = append([]string{exe}, strings.Fields(tt.cmdline[len(exe)+2:])...)
			}
			want := tt.want
			if want == nil {
				want = args
			}
			if got := rejoinProgram(tt.cmdline, tt.exe, args); !reflect.DeepEqual(got, want) {
				t.Errorf("rejoinProgram() = %q, want %q", got, want)
			}
		})
	}
}

func TestRejoinProgramKeepsArgsThatDisagreeWithTheLine(t *testing.T) {
	cmdline := "C:/A B/lgrassd.exe version"
	args := []string{"C:/A", "X/lgrassd.exe", "version"}
	if got := rejoinProgram(cmdline, `C:\A B\lgrassd.exe`, args); !reflect.DeepEqual(got, args) {
		t.Errorf("rejoinProgram() = %q, want args unchanged", got)
	}
}
