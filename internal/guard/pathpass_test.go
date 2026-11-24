// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package guard

import "testing"

func py(code string) string {
	return "python3 - <<'EOF'\n" + code + "\nEOF"
}

func nodeCode(code string) string {
	return "node <<'EOF'\n" + code + "\nEOF"
}

// pathPassAccesses are code that reads, writes or runs a guarded path. The path pass must refuse every one, whatever
// way of telling a mention from an access it uses.
var pathPassAccesses = []struct {
	command string
	want    string
}{
	{py(`import os; print(open(os.path.expanduser("~/.ssh/id_rsa")).read())`), "secret-path"},
	{py(`from pathlib import Path; print(Path("~/.ssh/config").expanduser().read_text())`), "secret-path"},
	{py("import os\nkey = \"~/.ssh/id_ed25519\"\ndata = open(os.path.expanduser(key)).read()"), "secret-path"},
	{py("import os\nprint(open(f\"{os.environ['HOME']}/.aws/credentials\").read())"), "secret-path"},
	{py("import os\np = os.path.join(os.path.expanduser('~'), '.ssh', 'id_rsa')\nprint(open(p).read())"), "secret-path"},
	{py("import os\ncmd = \"cat ~/.ssh/id_rsa\"\nos.system(cmd)"), "secret-path"},
	{py(`import os, shutil; shutil.copy(os.path.expanduser("~/.ssh/id_rsa"), "key.txt")`), "secret-path"},
	{py(`import glob, os; print(glob.glob(os.path.expanduser("~/.ssh/*")))`), "secret-path"},
	{py(`import os, sqlite3; sqlite3.connect(os.path.expanduser("~/.lemongrass/lemongrass.db"))`), "secret-path"},
	{py(`helper("~/.ssh/id_rsa")`), "secret-path"},
	{py("exec(code)\nnote = 'see ~/.ssh/config'"), "secret-path"},
	{py(`import os; open(os.path.expanduser("~/.claude/settings.json"), "w").write("{}")`), "guard-config"},
	{py("import os\nwith open(os.path.expanduser('~/.bashrc'), 'a') as f:\n    f.write('x')"), "persistence-write"},
	{py(`open("biblio/toolbox/x/q.sql", "w").write("select 1")`), "toolbox-write"},
	{nodeCode(`const fs = require("fs"), os = require("os"), path = require("path"); console.log(fs.readFileSync(path.join(os.homedir(), ".ssh", "id_rsa"), "utf8"))`), "secret-path"},
	{nodeCode("const fs = require('fs'), os = require('os');\nconst p = `${os.homedir()}/.aws/credentials`;\nconsole.log(fs.readFileSync(p, 'utf8'))"), "secret-path"},
	{nodeCode(`require("fs").writeFileSync(require("os").homedir() + "/.claude/settings.json", "{}")`), "guard-config"},
}

// pathPassMentions name a guarded path without opening it, or only read one that a read may reach. The mention scan
// refused each of them.
var pathPassMentions = []string{
	py("def sub(p, old, new):\n    s = open(p).read()\n    assert s.count(old) == 1, (p, old)\n    open(p, 'w').write(s.replace(old, new))\nsub('notes.md', 'kept in ~/.lemongrass/rester-client.json', 'kept per request')"),
	py("p = 'prd.md'\ns = open(p).read()\ns = s.replace('see ~/.ssh/config', 'see the ssh config')\nopen(p, 'w').write(s)"),
	py(`print("see ~/.ssh/config")`),
	py(`open("notes.md", "w").write("the key lives at ~/.ssh/id_rsa")`),
	py("reps = [('a ~/.lemongrass/x', 'b'), ('c', 'd ~/.aws/credentials')]\np = 'doc.md'\ns = open(p).read()\nfor a, b in reps:\n    s = s.replace(a, b)\nopen(p, 'w').write(s)"),
	py("import sys\nfor line in sys.stdin:\n    if '~/.ssh/' in line:\n        print(line)"),
	py("# reads ~/.ssh/config\nprint(1)"),
	py(`import json, os; print(json.load(open(os.path.expanduser("~/.claude/settings.json"))))`),
	py(`print(open("biblio/toolbox/x/q.sql").read())`),
	py("name = 'x'\nprint(f'{name} keeps its key in ~/.ssh')"),
	py("def f():\n    'Reads nothing from ~/.ssh/config.'\n    return 1\nf()"),
	py(`import json; json.dump({"path": "~/.ssh/config"}, open("out.json", "w"))`),
	py("import re\np = 'a.go'\ns = open(p).read()\ns = re.sub('old', 'filepath.Join(home, \".lemongrass\", \"x.json\")', s)\nopen(p, 'w').write(s)"),
	py("from pathlib import Path\np = Path('doc.md')\np.write_text(p.read_text().replace('~/.lemongrass/x', 'y'))"),
	py("lines = []\nlines.append('see ~/.ssh/config')\nprint(''.join(lines))"),
	py(`open("biblio/scratchpad/t/notes.md", "a").write("biblio/toolbox/ holds scripts")`),
}

func TestPathPassLetsMentionsAndAllowedReadsPass(t *testing.T) {
	root, home, _ := rmProject(t)
	for _, command := range pathPassMentions {
		if got := ruleOf(decideIn(t, root, home, root, command, Policy{})); got != "" {
			t.Errorf("%q: got %q, want it to pass", command, got)
		}
	}
}

func TestPathPassRefusesAccesses(t *testing.T) {
	root, home, _ := rmProject(t)
	for _, c := range pathPassAccesses {
		if got := ruleOf(decideIn(t, root, home, root, c.command, Policy{})); got != c.want {
			t.Errorf("%q: got %q, want %q", c.command, got, c.want)
		}
	}
}
