// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package workgroup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validYAML = `
name: schema-review
leader_label: lead
thinkers:
  - label: reviewer
    vendor: claude
    model: sonnet
    prompt: |
      Review the schema changes.
  - label: tester
    vendor: codex
    prompt: Write the tests.
`

func TestParseYAMLAppliesFields(t *testing.T) {
	cfg, err := Parse([]byte(validYAML), t.TempDir())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Name != "schema-review" || cfg.LeaderLabel != "lead" || len(cfg.Thinkers) != 2 {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.Thinkers[0].Prompt != "Review the schema changes." || cfg.Thinkers[0].Model != "sonnet" || cfg.Thinkers[1].Vendor != "codex" {
		t.Errorf("thinkers = %+v", cfg.Thinkers)
	}
}

func TestParseAcceptsJSONAndDefaults(t *testing.T) {
	cfg, err := Parse([]byte(`{"name":"audit","thinkers":[{"label":"a","vendor":"claude","prompt":"go"}]}`), t.TempDir())
	if err != nil {
		t.Fatalf("Parse JSON: %v", err)
	}
	if cfg.Name != "audit" || cfg.LeaderLabel != "leader" {
		t.Errorf("parsed = %q %q, want audit leader", cfg.Name, cfg.LeaderLabel)
	}
}

func TestParseRequiresAName(t *testing.T) {
	for _, body := range []string{
		`thinkers: [{label: a, vendor: claude, prompt: go}]`,
		`{"name":"   ","thinkers":[{"label":"a","vendor":"claude","prompt":"go"}]}`,
	} {
		_, err := Parse([]byte(body), t.TempDir())
		if err == nil || !strings.HasPrefix(err.Error(), "workgroup: ") || !strings.Contains(err.Error(), "needs a name") {
			t.Errorf("Parse(%q) error = %v, want a workgroup: error asking for a name", body, err)
		}
	}
}

func TestParseReadsPromptFileRelativeToTheConfig(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "prompts"), 0o755)
	os.WriteFile(filepath.Join(dir, "prompts", "a.md"), []byte("from a file\n"), 0o600)
	path := filepath.Join(dir, "team.yaml")
	os.WriteFile(path, []byte("name: t\nthinkers:\n  - {label: a, vendor: codex, prompt_file: prompts/a.md}\n"), 0o600)

	cfg, err := Load(path)
	if err != nil || cfg.Thinkers[0].Prompt != "from a file" || cfg.Thinkers[0].PromptFile != "" {
		t.Fatalf("Load = %+v, %v, want the file's text as the prompt", cfg, err)
	}
}

func TestParseRejectsInvalidConfigs(t *testing.T) {
	long := strings.Repeat("x", MaxPromptRunes+1)
	cases := map[string]string{
		"no thinkers":        "name: t\nthinkers: []",
		"unknown key":        "name: t\nthinkers:\n  - {label: a, vendor: claude, prompt: x, extra: 1}",
		"bad vendor":         "name: t\nthinkers:\n  - {label: a, vendor: cursor, prompt: x}",
		"bad label":          "name: t\nthinkers:\n  - {label: 'has space', vendor: claude, prompt: x}",
		"duplicate label":    "name: t\nthinkers:\n  - {label: a, vendor: claude, prompt: x}\n  - {label: A, vendor: codex, prompt: y}",
		"leader label clash": "name: t\nleader_label: a\nthinkers:\n  - {label: a, vendor: claude, prompt: x}",
		"bad model":          "name: t\nthinkers:\n  - {label: a, vendor: claude, model: 'x y', prompt: x}",
		"both prompts":       "name: t\nthinkers:\n  - {label: a, vendor: claude, prompt: x, prompt_file: y.md}",
		"no prompt":          "name: t\nthinkers:\n  - {label: a, vendor: claude}",
		"missing file":       "name: t\nthinkers:\n  - {label: a, vendor: claude, prompt_file: nope.md}",
		"prompt too long":    "name: t\nthinkers:\n  - {label: a, vendor: claude, prompt: " + long + "}",
		"not yaml":           "name: t\nthinkers: [unclosed",
	}
	for name, body := range cases {
		if _, err := Parse([]byte(body), t.TempDir()); err == nil {
			t.Errorf("%s accepted", name)
		} else if !strings.HasPrefix(err.Error(), "workgroup: ") {
			t.Errorf("%s error %q lacks the package prefix", name, err)
		}
	}
}

func TestSkillsAreDedupedAndTheThinkerSkillIsImplicit(t *testing.T) {
	cfg, err := Parse([]byte("name: t\nthinkers:\n  - {label: a, vendor: codex, prompt: x, skills: [lgrass-connector, lgrass-howtobe-thinker, bibliothek, lgrass-connector]}"), t.TempDir())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	c := cfg.Thinkers[0]
	if got := c.RequiredSkills(); len(got) != 3 || got[0] != "lgrass-howtobe-thinker" || got[1] != "lgrass-connector" || got[2] != "bibliothek" {
		t.Errorf("RequiredSkills = %v, want the thinker skill first then the listed ones once each", got)
	}
	claude, _ := Parse([]byte("name: t\nthinkers:\n  - {label: a, vendor: claude, prompt: x, skills: [bibliothek, lgrass-howtobe-thinker, extra-skill]}"), t.TempDir())
	if got := claude.Thinkers[0].RequiredSkills(); len(got) != 1 || got[0] != "extra-skill" {
		t.Errorf("RequiredSkills of a Claude thinker = %v, want bibliothek and the thinker skill left out, since its prompt carries one and lgrass how thinker start replaces the other", got)
	}
	plain, _ := Parse([]byte("name: t\nthinkers:\n  - {label: a, vendor: codex, prompt: x}"), t.TempDir())
	if got := plain.Thinkers[0].RequiredSkills(); len(got) != 1 || got[0] != "lgrass-howtobe-thinker" {
		t.Errorf("RequiredSkills of a Codex thinker without a list = %v", got)
	}
}

func TestARetiredSkillIsRefusedForClaudeOnly(t *testing.T) {
	_, err := Parse([]byte("name: t\nthinkers:\n  - {label: a, vendor: claude, prompt: x, skills: [lgrass-howtodo-work]}"), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "lgrass how work") || !strings.HasPrefix(err.Error(), "workgroup: ") {
		t.Errorf("err = %v, want a refusal naming lgrass how work", err)
	}
	if _, err := Parse([]byte("name: t\nthinkers:\n  - {label: a, vendor: codex, prompt: x, skills: [lgrass-howtodo-work]}"), t.TempDir()); err != nil {
		t.Errorf("a Codex thinker was refused a skill it still has: %v", err)
	}
}

func TestSkillNamesAreValidated(t *testing.T) {
	for name, body := range map[string]string{
		"path in name": "name: t\nthinkers:\n  - {label: a, vendor: claude, prompt: x, skills: ['../evil']}",
		"space":        "name: t\nthinkers:\n  - {label: a, vendor: claude, prompt: x, skills: ['a b']}",
		"too many":     "name: t\nthinkers:\n  - {label: a, vendor: claude, prompt: x, skills: [a1, a2, a3, a4, a5, a6, a7, a8, a9]}",
	} {
		if _, err := Parse([]byte(body), t.TempDir()); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestCheckSkillsInstalledPerVendor(t *testing.T) {
	home := t.TempDir()
	install := func(vendorDir, name string) {
		dir := filepath.Join(home, vendorDir, "skills", name)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("x"), 0o600)
	}
	install(".claude", "lgrass-howtobe-thinker")
	install(".claude", "bibliothek")
	install(".codex", "lgrass-howtobe-thinker")

	cfg, _ := Parse([]byte("name: t\nthinkers:\n  - {label: a, vendor: claude, prompt: x, skills: [bibliothek]}\n  - {label: b, vendor: codex, prompt: y}"), t.TempDir())
	if err := cfg.CheckSkillsInstalled(home); err != nil {
		t.Fatalf("CheckSkillsInstalled: %v", err)
	}

	cfg, _ = Parse([]byte("name: t\nthinkers:\n  - {label: b, vendor: codex, prompt: y, skills: [bibliothek]}"), t.TempDir())
	err := cfg.CheckSkillsInstalled(home)
	if err == nil || !strings.Contains(err.Error(), `"bibliothek"`) || !strings.Contains(err.Error(), "lgrass-howtobe-thinker") {
		t.Errorf("error = %v, want the missing codex skill named and the installed ones listed", err)
	}
	if err := (Config{Thinkers: []Thinker{{Label: "a", Vendor: "codex"}}}).CheckSkillsInstalled(t.TempDir()); err == nil {
		t.Error("a home with no skills passed the check for a Codex thinker")
	}
	if err := (Config{Thinkers: []Thinker{{Label: "a", Vendor: "claude"}}}).CheckSkillsInstalled(t.TempDir()); err != nil {
		t.Errorf("a Claude thinker with no listed skills needs nothing installed: %v", err)
	}
}

func TestParseAdditionTakesOnlyThinkersWithFreshLabels(t *testing.T) {
	taken := []string{"lead", "reviewer"}
	cfg, err := ParseAddition([]byte("thinkers:\n  - {label: tester, vendor: codex, prompt: Write tests.}\n  - {label: docs, vendor: claude, prompt: Write docs.}"), t.TempDir(), taken)
	if err != nil {
		t.Fatalf("ParseAddition: %v", err)
	}
	if len(cfg.Thinkers) != 2 || cfg.Thinkers[0].Label != "tester" || cfg.Name != "" {
		t.Errorf("cfg = %+v", cfg)
	}

	cases := map[string]string{
		"name given":      "name: other\nthinkers:\n  - {label: a, vendor: claude, prompt: x}",
		"leader given":    "leader_label: boss\nthinkers:\n  - {label: a, vendor: claude, prompt: x}",
		"no thinkers":     "thinkers: []",
		"label taken":     "thinkers:\n  - {label: Reviewer, vendor: claude, prompt: x}",
		"leader taken":    "thinkers:\n  - {label: lead, vendor: claude, prompt: x}",
		"duplicate label": "thinkers:\n  - {label: a, vendor: claude, prompt: x}\n  - {label: a, vendor: codex, prompt: y}",
	}
	for name, body := range cases {
		if _, err := ParseAddition([]byte(body), t.TempDir(), taken); err == nil {
			t.Errorf("%s accepted", name)
		} else if !strings.HasPrefix(err.Error(), "workgroup: ") {
			t.Errorf("%s error %q lacks the package prefix", name, err)
		}
	}
}

func TestParseHasNoUpperThinkerLimit(t *testing.T) {
	var body strings.Builder
	body.WriteString("name: big\nthinkers:\n")
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&body, "  - {label: t%d, vendor: claude, prompt: x}\n", i)
	}
	cfg, err := Parse([]byte(body.String()), t.TempDir())
	if err != nil || len(cfg.Thinkers) != 12 {
		t.Errorf("Parse of 12 thinkers = %d, %v", len(cfg.Thinkers), err)
	}
}

func TestCommandsAreNormalizedDedupedAndTheThinkerStartIsImplicit(t *testing.T) {
	yaml := "name: t\nthinkers:\n  - label: a\n    vendor: claude\n    prompt: x\n    commands:\n      - lgrass how thinker start\n      - '  lgrass   how work task \"export-cli\" '\n      - lgrass how work task export-cli\n      - lgrass rester ab12cd34 info\n"
	cfg, err := Parse([]byte(yaml), t.TempDir())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := cfg.Thinkers[0].RequiredCommands()
	want := []string{"lgrass how thinker start", "lgrass how work task export-cli", "lgrass rester ab12cd34 info"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("RequiredCommands = %q, want %q", got, want)
	}
	codex, _ := Parse([]byte("name: t\nthinkers:\n  - {label: a, vendor: codex, prompt: x, commands: [lgrass how book write]}"), t.TempDir())
	if got := codex.Thinkers[0].RequiredCommands(); len(got) != 1 || got[0] != "lgrass how book write" {
		t.Errorf("RequiredCommands of a Codex thinker = %q, want only the listed command, since it loads the thinker skill instead", got)
	}
}

func TestOnlyReadOnlyLgrassCommandsAreAccepted(t *testing.T) {
	accepted := []string{
		"lgrass how book write", "lgrass how work task some-slug", "lgrass thread read a1b2c3 --all", "lgrass thread list",
		"lgrass workgroup thread", "lgrass workgroup list", "lgrass session list", "lgrass session tabs list",
		"lgrass biblio log books", "lgrass biblio show 12 books/toc.md", "lgrass rester ab12cd34 users", "lgrass tips list", "lgrass version",
	}
	refused := []string{
		"ls", "lgrass", "/usr/bin/lgrass how book write", "lgrass how", "lgrass how book", "lgrass how book burn", "lgrass how nothing write",
		"lgrass sign i-write-memory", "lgrass replay 3", "lgrass workgroup create c.yaml", "lgrass workgroup disband 1", "lgrass workgroup add c.yaml",
		"lgrass thread post a1b2c3 hi", "lgrass thread create t c", "lgrass tips add x", "lgrass db ab12 --sql x",
		"lgrass rester ab12 get /x", "lgrass rester ab12 flush", "lgrass rester ab12 info --out f", "lgrass device tap --text x", "lgrass flutter test",
		"lgrass how book write; rm x", "lgrass how book write && ls", "lgrass how book write > f", "lgrass thread read $(id)", "lgrass how book write \\",
		"lgrass how work task " + strings.Repeat("x", MaxCommandRunes),
	}
	for _, c := range accepted {
		if _, err := checkCommand(c); err != nil {
			t.Errorf("%q refused: %v", c, err)
		}
	}
	for _, c := range refused {
		if _, err := checkCommand(c); err == nil {
			t.Errorf("%q accepted, want it refused", c)
		}
	}
}

func TestACommandListIsCappedAndARefusalNamesTheThinker(t *testing.T) {
	_, err := Parse([]byte("name: t\nthinkers:\n  - {label: a, vendor: claude, prompt: x, commands: [lgrass sign x]}"), t.TempDir())
	if err == nil || !strings.HasPrefix(err.Error(), "workgroup: ") || !strings.Contains(err.Error(), `thinker "a"`) || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("err = %v", err)
	}
	many := strings.Repeat(", lgrass version", MaxCommands+1)[2:]
	if _, err := Parse([]byte("name: t\nthinkers:\n  - {label: a, vendor: claude, prompt: x, commands: ["+many+"]}"), t.TempDir()); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("err = %v, want the command limit", err)
	}
}
