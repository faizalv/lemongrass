package guard

import (
	"os"
	"path/filepath"
	"testing"
)

func rmProject(t *testing.T) (root, home, outside string) {
	t.Helper()
	home = t.TempDir()
	root = filepath.Join(home, "work", "app")
	outside = t.TempDir()
	for _, dir := range []string{"build/out", "sub", ".git/refs", "biblio/books"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"a.log", "b.log", "notes.txt", "sub/c.txt", ".git/index.lock", "biblio/books/toc.md"} {
		if err := os.WriteFile(filepath.Join(root, file), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	return root, home, outside
}

func TestRmInsideProject(t *testing.T) {
	root, home, outside := rmProject(t)
	cases := []struct {
		command string
		cwd     string
		want    string
	}{
		{"rm notes.txt", "", ""},
		{"rm -rf build", "", ""},
		{"rm -f a.log b.log", "", ""},
		{"rm *.log", "", ""},
		{"rm -r -- build/out", "", ""},
		{"rm -rf " + filepath.Join(root, "build"), "", ""},
		{"rm -rf ~/work/app/build", "", ""},
		{"rm -rf $HOME/work/app/build", "", ""},
		{"rm -rf \"${HOME}/work/app/build\"", "", ""},
		{"rm -rf build && go build ./...", "", ""},
		{"bash -c 'rm -rf build'", "", ""},
		{"rm c.txt", "sub", ""},
		{"rm -rf .", "sub", ""},

		{"rm -rf " + outside, "", "rm-force"},
		{"rm " + filepath.Join(outside, "x"), "", "rm-plain"},
		{"rm -rf ~/other", "", "rm-force"},
		{"rm -rf ~", "", "rm-force"},
		{"rm -rf ~root/x", "", "rm-force"},
		{"rm -rf ../app", "", "rm-force"},
		{"rm -rf build/../..", "", "rm-force"},
		{"rm -rf escape", "", "rm-force"},
		{"rm -rf escape/x", "", "rm-force"},
		{"rm -rf .", "", "rm-force"},
		{"rm -rf " + root, "", "rm-force"},
		{"rm -rf /*", "", "rm-force"},
		{"rm -rf .git", "", "rm-force"},
		{"rm .git/index.lock", "", "rm-plain"},
		{"rm -rf biblio", "", "rm-force"},
		{"rm biblio/books/toc.md", "", "rm-plain"},
		{"rm -rf *", "", "rm-force"},
		{"rm -rf .*", "", "rm-force"},
		{"rm -rf \"$TMP\"/build", "", "rm-force"},
		{"rm -rf $(pwd)/build", "", "rm-force"},
		{"rm -rf {build,/etc}", "", "rm-force"},
		{"rm -rf --no-preserve-root build", "", "rm-force"},
		{"rm -rf", "", "rm-force"},
		{"cd build && rm -rf out", "", "rm-force"},
		{"(cd /tmp; rm -rf build)", "", "rm-force"},
		{"pushd sub && rm c.txt", "", "rm-plain"},
		{"env -C /tmp rm -rf build", "", "rm-force"},
		{"ls | xargs rm -f", "", "rm-force"},
		{"echo build | xargs rm -rf build", "", "rm-force"},
		{"find . -name '*.tmp' -exec rm {} \\;", "", "rm-plain"},
		{"rm -rf build", outside, "rm-force"},

		{"find build -name '*.o' -delete", "", ""},
		{"find build sub -type f -delete", "", ""},
		{"find -P build -delete", "", ""},
		{"find " + filepath.Join(root, "build") + " -delete", "", ""},
		{"find ~/work/app/build -delete", "", ""},
		{"find . -name '*.tmp' -delete", "sub", ""},
		{"find . -name '*.pyc' -delete", "", "find-delete"},
		{"find -name '*.pyc' -delete", "", "find-delete"},
		{"find " + root + " -delete", "", "find-delete"},
		{"find .git -name '*.lock' -delete", "", "find-delete"},
		{"find biblio -delete", "", "find-delete"},
		{"find build biblio -delete", "", "find-delete"},
		{"find " + outside + " -delete", "", "find-delete"},
		{"find escape -delete", "", "find-delete"},
		{"find -L build -delete", "", "find-delete"},
		{"find build -follow -delete", "", "find-delete"},
		{"find \"$DIR\" -delete", "", "find-delete"},
		{"cd build && find out -delete", "", "find-delete"},
		{"find build -delete", outside, "find-delete"},
		{"find build -name x -exec rm {} \\;", "", "rm-plain"},
	}
	for _, tc := range cases {
		t.Run(tc.command+" in "+tc.cwd, func(t *testing.T) {
			cwd := root
			if filepath.IsAbs(tc.cwd) {
				cwd = tc.cwd
			} else if tc.cwd != "" {
				cwd = filepath.Join(root, tc.cwd)
			}
			in := Input{Tool: "Bash", Command: tc.command, Cwd: cwd, Home: home, ProjectRoot: root}
			got := ""
			if verdict := Decide(in, Policy{}); verdict != nil {
				got = verdict.RuleID
			}
			if got != tc.want {
				t.Errorf("rule = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRmWithoutProjectRootKeepsGate(t *testing.T) {
	root, home, _ := rmProject(t)
	in := Input{Tool: "Bash", Command: "rm -rf build", Cwd: root, Home: home}
	if verdict := Decide(in, Policy{}); verdict == nil || verdict.RuleID != "rm-force" {
		t.Errorf("verdict = %+v, want rm-force", verdict)
	}
}

func TestRmInsideProjectHonorsUserDeny(t *testing.T) {
	root, home, _ := rmProject(t)
	in := Input{Tool: "Bash", Command: "rm notes.txt", Cwd: root, Home: home, ProjectRoot: root}
	policy := Policy{Rules: map[string]string{"rm-plain": overrideDeny}}
	if verdict := Decide(in, policy); verdict == nil || verdict.RuleID != "rm-plain" || verdict.Mode != ModeDeny {
		t.Errorf("verdict = %+v, want an rm-plain deny", verdict)
	}
}

func TestRmInsideProjectKeepsSecretCheck(t *testing.T) {
	root, home, _ := rmProject(t)
	if err := os.MkdirAll(filepath.Join(root, ".ssh"), 0o755); err != nil {
		t.Fatal(err)
	}
	in := Input{Tool: "Bash", Command: "rm -rf .ssh", Cwd: root, Home: home, ProjectRoot: root}
	if verdict := Decide(in, Policy{}); verdict == nil || verdict.RuleID != "secret-path" {
		t.Errorf("verdict = %+v, want secret-path", verdict)
	}
}
