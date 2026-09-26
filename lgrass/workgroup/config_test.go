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
	cfg, err := Parse([]byte(`{"copilots":[{"label":"a","vendor":"claude","prompt":"go"}]}`), t.TempDir())
	if err != nil {
		t.Fatalf("Parse JSON: %v", err)
	}
	if cfg.Name != "workgroup" || cfg.PilotLabel != "pilot" {
		t.Errorf("defaults = %q %q, want workgroup pilot", cfg.Name, cfg.PilotLabel)
	}
}

func TestParseReadsPromptFileRelativeToTheConfig(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "prompts"), 0o755)
	os.WriteFile(filepath.Join(dir, "prompts", "a.md"), []byte("from a file\n"), 0o600)
	path := filepath.Join(dir, "team.yaml")
	os.WriteFile(path, []byte("copilots:\n  - {label: a, vendor: codex, prompt_file: prompts/a.md}\n"), 0o600)

	cfg, err := Load(path)
	if err != nil || cfg.Copilots[0].Prompt != "from a file" || cfg.Copilots[0].PromptFile != "" {
		t.Fatalf("Load = %+v, %v, want the file's text as the prompt", cfg, err)
	}
}

func TestParseRejectsInvalidConfigs(t *testing.T) {
	long := strings.Repeat("x", MaxPromptRunes+1)
	cases := map[string]string{
		"no copilots":       "copilots: []",
		"unknown key":       "copilots:\n  - {label: a, vendor: claude, prompt: x, extra: 1}",
		"bad vendor":        "copilots:\n  - {label: a, vendor: cursor, prompt: x}",
		"bad label":         "copilots:\n  - {label: 'has space', vendor: claude, prompt: x}",
		"duplicate label":   "copilots:\n  - {label: a, vendor: claude, prompt: x}\n  - {label: A, vendor: codex, prompt: y}",
		"pilot label clash": "pilot_label: a\ncopilots:\n  - {label: a, vendor: claude, prompt: x}",
		"bad model":         "copilots:\n  - {label: a, vendor: claude, model: 'x y', prompt: x}",
		"both prompts":      "copilots:\n  - {label: a, vendor: claude, prompt: x, prompt_file: y.md}",
		"no prompt":         "copilots:\n  - {label: a, vendor: claude}",
		"missing file":      "copilots:\n  - {label: a, vendor: claude, prompt_file: nope.md}",
		"prompt too long":   "copilots:\n  - {label: a, vendor: claude, prompt: " + long + "}",
		"too many":          "copilots:\n" + strings.Repeat("  - {label: a, vendor: claude, prompt: x}\n", MaxCopilots+1),
		"not yaml":          "copilots: [unclosed",
	}
	for name, body := range cases {
		if _, err := Parse([]byte(body), t.TempDir()); err == nil {
			t.Errorf("%s accepted", name)
		} else if !strings.HasPrefix(err.Error(), "workgroup: ") {
			t.Errorf("%s error %q lacks the package prefix", name, err)
		}
	}
}
