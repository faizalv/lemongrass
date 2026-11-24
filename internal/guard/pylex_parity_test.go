// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package guard

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pyStringsScript prints every str literal of the program on stdin as Python's own parser reads it: implicit
// concatenation joined, and an f-string as its literal parts with the replacement fields left out.
const pyStringsScript = `
import ast, json, sys
tree = ast.parse(sys.stdin.read())
out, skip = [], set()
for node in ast.walk(tree):
    if id(node) in skip:
        continue
    if isinstance(node, ast.JoinedStr):
        out.append("".join(v.value for v in node.values if isinstance(v, ast.Constant)))
        for inner in ast.walk(node):
            skip.add(id(inner))
    elif isinstance(node, ast.Constant) and isinstance(node.value, str):
        out.append(node.value)
print(json.dumps(out))
`

// pythonStrings runs the host's Python on code; ok is false when Python is missing or rejects the code.
func pythonStrings(t *testing.T, python, code string) ([]string, bool) {
	cmd := exec.Command(python, "-I", "-c", pyStringsScript)
	cmd.Stdin = strings.NewReader(code)
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	var values []string
	if err := json.Unmarshal(out, &values); err != nil {
		t.Fatalf("reading the strings Python printed: %v", err)
	}
	return values, true
}

// checkPyParity checks that every string Python reads in code is a literal the guard's parser reads too.
func checkPyParity(t *testing.T, python, code string) (checked, mismatched bool) {
	want, ok := pythonStrings(t, python, code)
	if !ok {
		return false, false
	}
	prog, err := pyParse(code)
	if err != nil {
		t.Errorf("Python reads this code but the guard's parser does not (%v):\n%s", err, code)
		return true, true
	}
	have := map[string]int{}
	for _, lit := range prog.lits {
		have[lit.value]++
	}
	for _, value := range want {
		if have[value] == 0 {
			t.Errorf("Python reads the string %q, the guard's parser does not, in:\n%.400s", value, code)
			return true, true
		}
		have[value]--
	}
	return true, false
}

func pyFixturePrograms(t *testing.T) []string {
	var out []string
	commands := append([]string(nil), pathPassMentions...)
	for _, c := range pathPassAccesses {
		commands = append(commands, c.command)
	}
	for _, command := range commands {
		script, err := parseShell(command)
		if err != nil {
			t.Fatalf("%q: %v", command, err)
		}
		for _, p := range script.Programs {
			if p.Lang == "python" {
				out = append(out, p.Code)
			}
		}
	}
	return out
}

// TestPyLexerParity checks the guard's Python strings against the host's Python on the fixtures, and on the replay
// corpus when LGRASS_GUARD_REPLAY is set. It is skipped without a python3.
func TestPyLexerParity(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not installed")
	}
	programs := pyFixturePrograms(t)
	if env := os.Getenv(replayEnv); env != "" {
		corpus, _ := replayPrograms(t, filepath.SplitList(env))
		for _, p := range corpus {
			if p.lang == "python" {
				programs = append(programs, p.code)
			}
		}
	}
	checked, mismatched := 0, 0
	for _, code := range programs {
		c, m := checkPyParity(t, python, code)
		if c {
			checked++
		}
		if m {
			mismatched++
		}
		if mismatched >= 10 {
			break
		}
	}
	t.Logf("programs Python reads: %d, mismatched: %d", checked, mismatched)
}
