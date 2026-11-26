// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package guard

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestTouchedPaths(t *testing.T) {
	home := t.TempDir()
	cwd := resolveFully(t.TempDir())
	other := resolveFully(t.TempDir())
	cases := []struct {
		name string
		in   Input
		want []string
		not  []string
	}{
		{"cwd alone", Input{Tool: "Bash", Command: "ls", Cwd: cwd}, []string{cwd}, nil},
		{"literal cd", Input{Tool: "Bash", Command: "cd " + other + " && make", Cwd: cwd}, []string{cwd, other}, nil},
		{"absolute argument", Input{Tool: "Bash", Command: "cat " + filepath.Join(other, "a.txt"), Cwd: cwd}, []string{filepath.Join(other, "a.txt")}, nil},
		{"value of a long flag", Input{Tool: "Bash", Command: "tool --file=" + filepath.Join(other, "b"), Cwd: cwd}, []string{filepath.Join(other, "b")}, nil},
		{"redirect", Input{Tool: "Bash", Command: "echo x > " + filepath.Join(other, "out"), Cwd: cwd}, []string{filepath.Join(other, "out")}, nil},
		{"relative argument after cd", Input{Tool: "Bash", Command: "cd " + other + " && cat sub/c", Cwd: cwd}, []string{filepath.Join(other, "sub/c")}, nil},
		{"variable cd adds nothing", Input{Tool: "Bash", Command: "cd $DIR && ls", Cwd: cwd}, []string{cwd}, []string{"$DIR"}},
		{"tool path", Input{Tool: "Write", Paths: []string{filepath.Join(other, "d")}, Cwd: cwd}, []string{cwd, filepath.Join(other, "d")}, nil},
		{"unparseable command keeps the cwd", Input{Tool: "Bash", Command: "echo 'open", Cwd: cwd}, []string{cwd}, nil},
	}
	for _, tc := range cases {
		tc.in.Home = home
		got := TouchedPaths(tc.in)
		for _, w := range tc.want {
			if !slices.Contains(got, w) {
				t.Errorf("%s: %v lacks %s", tc.name, got, w)
			}
		}
		for _, n := range tc.not {
			for _, g := range got {
				if filepath.Base(g) == n {
					t.Errorf("%s: %v holds %s", tc.name, got, n)
				}
			}
		}
	}
}
