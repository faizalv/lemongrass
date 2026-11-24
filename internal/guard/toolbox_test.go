// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"os"
	"path/filepath"
	"testing"
)

// Every write into biblio/toolbox/ is refused so `lgrass toolbox` stays the only way in, while an item can still be
// read, run and copied out.
func TestToolboxWritesAreRefusedAndReadsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "biblio/toolbox/x"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"run.sh", "q.py"} {
		if err := os.WriteFile(filepath.Join(root, "biblio/toolbox/x", name), []byte("print(1)\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	judge := func(in Input) string {
		in.Cwd, in.ProjectRoot, in.Home = root, root, t.TempDir()
		if v := Decide(in, Policy{}); v != nil {
			return v.RuleID
		}
		return "pass"
	}
	for _, in := range []Input{
		{Tool: "Write", Paths: []string{filepath.Join(root, "biblio/toolbox/x/run.sh")}, Writes: true},
		{Tool: "Edit", Paths: []string{"biblio/toolbox/inventory.md"}, Writes: true},
		{Tool: "Bash", Command: "cp a.sh biblio/toolbox/x/"},
		{Tool: "Bash", Command: "cp -t biblio/toolbox/x a.sh"},
		{Tool: "Bash", Command: "cp biblio/toolbox/x/run.sh biblio/toolbox/y/"},
		{Tool: "Bash", Command: "echo x >> biblio/toolbox/inventory.md"},
		{Tool: "Bash", Command: "sed -i s/a/b/ biblio/toolbox/x/run.sh"},
		{Tool: "Bash", Command: "mkdir biblio/toolbox/y"},
		{Tool: "Bash", Command: "mv biblio/toolbox/x/run.sh /tmp/"},
		{Tool: "Bash", Command: "cp a.sh biblio/toolbox/x/run.sh && bash biblio/toolbox/x/run.sh"},
	} {
		if got := judge(in); got != toolboxWriteRule {
			t.Errorf("%s %q %v = %s, want %s", in.Tool, in.Command, in.Paths, got, toolboxWriteRule)
		}
	}
	for _, in := range []Input{
		{Tool: "Read", Paths: []string{"biblio/toolbox/x/run.sh"}},
		{Tool: "Bash", Command: "cat biblio/toolbox/x/run.sh"},
		{Tool: "Bash", Command: "grep -r hi biblio/toolbox"},
		{Tool: "Bash", Command: "bash biblio/toolbox/x/run.sh"},
		{Tool: "Bash", Command: "./biblio/toolbox/x/run.sh"},
		{Tool: "Bash", Command: "python3 biblio/toolbox/x/q.py"},
		{Tool: "Bash", Command: "sh biblio/toolbox/x/run.sh > out.txt"},
		{Tool: "Bash", Command: "cp biblio/toolbox/x/run.sh out.sh"},
		{Tool: "Bash", Command: "rsync -a biblio/toolbox/x/ copy/"},
		{Tool: "Bash", Command: "lgrass toolbox add y --about 'Does a thing.' biblio/scratchpad/t/a.sh"},
	} {
		if got := judge(in); got != "pass" {
			t.Errorf("%s %q %v = %s, want pass", in.Tool, in.Command, in.Paths, got)
		}
	}
}
