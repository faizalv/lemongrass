package workgroup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validYAML = `
name: schema-review
pilot_label: lead
copilots:
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
	if cfg.Name != "schema-review" || cfg.PilotLabel != "lead" || len(cfg.Copilots) != 2 {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.Copilots[0].Prompt != "Review the schema changes." || cfg.Copilots[0].Model != "sonnet" || cfg.Copilots[1].Vendor != "codex" {
		t.Errorf("copilots = %+v", cfg.Copilots)
	}
}

func TestParseAcceptsJSONAndDefaults(t *testing.T) {
	cfg, err := Parse([]byte(`{"name":"audit","copilots":[{"label":"a","vendor":"claude","prompt":"go"}]}`), t.TempDir())
	if err != nil {
		t.Fatalf("Parse JSON: %v", err)
	}
	if cfg.Name != "audit" || cfg.PilotLabel != "pilot" {
		t.Errorf("parsed = %q %q, want audit pilot", cfg.Name, cfg.PilotLabel)
	}
}

func TestParseRequiresAName(t *testing.T) {
	for _, body := range []string{
		`copilots: [{label: a, vendor: claude, prompt: go}]`,
		`{"name":"   ","copilots":[{"label":"a","vendor":"claude","prompt":"go"}]}`,
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
	os.WriteFile(path, []byte("name: t\ncopilots:\n  - {label: a, vendor: codex, prompt_file: prompts/a.md}\n"), 0o600)

	cfg, err := Load(path)
	if err != nil || cfg.Copilots[0].Prompt != "from a file" || cfg.Copilots[0].PromptFile != "" {
		t.Fatalf("Load = %+v, %v, want the file's text as the prompt", cfg, err)
	}
}

func TestParseRejectsInvalidConfigs(t *testing.T) {
	long := strings.Repeat("x", MaxPromptRunes+1)
	cases := map[string]string{
		"no copilots":       "name: t\ncopilots: []",
		"unknown key":       "name: t\ncopilots:\n  - {label: a, vendor: claude, prompt: x, extra: 1}",
		"bad vendor":        "name: t\ncopilots:\n  - {label: a, vendor: cursor, prompt: x}",
		"bad label":         "name: t\ncopilots:\n  - {label: 'has space', vendor: claude, prompt: x}",
		"duplicate label":   "name: t\ncopilots:\n  - {label: a, vendor: claude, prompt: x}\n  - {label: A, vendor: codex, prompt: y}",
		"pilot label clash": "name: t\npilot_label: a\ncopilots:\n  - {label: a, vendor: claude, prompt: x}",
		"bad model":         "name: t\ncopilots:\n  - {label: a, vendor: claude, model: 'x y', prompt: x}",
		"both prompts":      "name: t\ncopilots:\n  - {label: a, vendor: claude, prompt: x, prompt_file: y.md}",
		"no prompt":         "name: t\ncopilots:\n  - {label: a, vendor: claude}",
		"missing file":      "name: t\ncopilots:\n  - {label: a, vendor: claude, prompt_file: nope.md}",
		"prompt too long":   "name: t\ncopilots:\n  - {label: a, vendor: claude, prompt: " + long + "}",
		"too many":          "name: t\ncopilots:\n" + strings.Repeat("  - {label: a, vendor: claude, prompt: x}\n", MaxCopilots+1),
		"not yaml":          "name: t\ncopilots: [unclosed",
	}
	for name, body := range cases {
		if _, err := Parse([]byte(body), t.TempDir()); err == nil {
			t.Errorf("%s accepted", name)
		} else if !strings.HasPrefix(err.Error(), "workgroup: ") {
			t.Errorf("%s error %q lacks the package prefix", name, err)
		}
	}
}

func TestSkillsAreDedupedAndTheCopilotSkillIsImplicit(t *testing.T) {
	cfg, err := Parse([]byte("name: t\ncopilots:\n  - {label: a, vendor: claude, prompt: x, skills: [lgrass-connector, lgrass-copilot, bibliothek, lgrass-connector]}"), t.TempDir())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	c := cfg.Copilots[0]
	if got := c.RequiredSkills(); len(got) != 3 || got[0] != "lgrass-copilot" || got[1] != "lgrass-connector" || got[2] != "bibliothek" {
		t.Errorf("RequiredSkills = %v, want the copilot skill first then the listed ones once each", got)
	}
	plain, _ := Parse([]byte("name: t\ncopilots:\n  - {label: a, vendor: claude, prompt: x}"), t.TempDir())
	if got := plain.Copilots[0].RequiredSkills(); len(got) != 1 || got[0] != "lgrass-copilot" {
		t.Errorf("RequiredSkills without a list = %v", got)
	}
}

func TestSkillNamesAreValidated(t *testing.T) {
	for name, body := range map[string]string{
		"path in name": "name: t\ncopilots:\n  - {label: a, vendor: claude, prompt: x, skills: ['../evil']}",
		"space":        "name: t\ncopilots:\n  - {label: a, vendor: claude, prompt: x, skills: ['a b']}",
		"too many":     "name: t\ncopilots:\n  - {label: a, vendor: claude, prompt: x, skills: [a1, a2, a3, a4, a5, a6, a7, a8, a9]}",
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
	install(".claude", "lgrass-copilot")
	install(".claude", "bibliothek")
	install(".codex", "lgrass-copilot")

	cfg, _ := Parse([]byte("name: t\ncopilots:\n  - {label: a, vendor: claude, prompt: x, skills: [bibliothek]}\n  - {label: b, vendor: codex, prompt: y}"), t.TempDir())
	if err := cfg.CheckSkillsInstalled(home); err != nil {
		t.Fatalf("CheckSkillsInstalled: %v", err)
	}

	cfg, _ = Parse([]byte("name: t\ncopilots:\n  - {label: b, vendor: codex, prompt: y, skills: [bibliothek]}"), t.TempDir())
	err := cfg.CheckSkillsInstalled(home)
	if err == nil || !strings.Contains(err.Error(), `"bibliothek"`) || !strings.Contains(err.Error(), "lgrass-copilot") {
		t.Errorf("error = %v, want the missing codex skill named and the installed ones listed", err)
	}
	if err := (Config{Copilots: []Copilot{{Label: "a", Vendor: "claude"}}}).CheckSkillsInstalled(t.TempDir()); err == nil {
		t.Error("a home with no skills passed the check")
	}
}
