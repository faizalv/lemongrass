// Package workgroup parses workgroup configs and talks to the lemongrass app that spawns a group's tabs.
package workgroup

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	MaxCopilots      = 5
	MaxPromptRunes   = 3000
	MaxNameRunes     = 120
	defaultName      = "workgroup"
	defaultPilotName = "pilot"
)

var (
	labelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,29}$`)
	modelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,79}$`)
	vendors      = map[string]bool{"claude": true, "codex": true}
)

type Copilot struct {
	Label      string `yaml:"label"`
	Vendor     string `yaml:"vendor"`
	Model      string `yaml:"model"`
	Prompt     string `yaml:"prompt"`
	PromptFile string `yaml:"prompt_file"`
}

type Config struct {
	Name       string    `yaml:"name"`
	PilotLabel string    `yaml:"pilot_label"`
	Copilots   []Copilot `yaml:"copilots"`
}

// JSON is accepted because it is valid YAML. Unknown keys are an error, and each prompt_file is read relative to the config's directory into Prompt.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("workgroup: reading config: %w", err)
	}
	return Parse(data, filepath.Dir(path))
}

func Parse(data []byte, baseDir string) (Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("workgroup: parsing config: %w", err)
	}

	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.Name == "" {
		cfg.Name = defaultName
	}
	if utf8.RuneCountInString(cfg.Name) > MaxNameRunes {
		return Config{}, fmt.Errorf("workgroup: name is longer than %d characters", MaxNameRunes)
	}
	if cfg.PilotLabel == "" {
		cfg.PilotLabel = defaultPilotName
	}
	if !labelPattern.MatchString(cfg.PilotLabel) {
		return Config{}, fmt.Errorf("workgroup: pilot_label %q must be 1 to 30 letters, digits, dashes or underscores", cfg.PilotLabel)
	}
	if len(cfg.Copilots) == 0 {
		return Config{}, errors.New("workgroup: the config lists no copilots")
	}
	if len(cfg.Copilots) > MaxCopilots {
		return Config{}, fmt.Errorf("workgroup: %d copilots listed and the limit is %d", len(cfg.Copilots), MaxCopilots)
	}

	seen := map[string]bool{strings.ToLower(cfg.PilotLabel): true}
	for i := range cfg.Copilots {
		c := &cfg.Copilots[i]
		if !labelPattern.MatchString(c.Label) {
			return Config{}, fmt.Errorf("workgroup: copilot %d label %q must be 1 to 30 letters, digits, dashes or underscores", i+1, c.Label)
		}
		if seen[strings.ToLower(c.Label)] {
			return Config{}, fmt.Errorf("workgroup: label %q is used more than once", c.Label)
		}
		seen[strings.ToLower(c.Label)] = true
		if !vendors[c.Vendor] {
			return Config{}, fmt.Errorf("workgroup: copilot %q vendor %q must be claude or codex", c.Label, c.Vendor)
		}
		if c.Model != "" && !modelPattern.MatchString(c.Model) {
			return Config{}, fmt.Errorf("workgroup: copilot %q model %q is not a valid model name", c.Label, c.Model)
		}
		if (c.Prompt == "") == (c.PromptFile == "") {
			return Config{}, fmt.Errorf("workgroup: copilot %q needs exactly one of prompt and prompt_file", c.Label)
		}
		if c.PromptFile != "" {
			file := c.PromptFile
			if !filepath.IsAbs(file) {
				file = filepath.Join(baseDir, file)
			}
			text, err := os.ReadFile(file)
			if err != nil {
				return Config{}, fmt.Errorf("workgroup: copilot %q prompt_file: %w", c.Label, err)
			}
			c.Prompt, c.PromptFile = string(text), ""
		}
		c.Prompt = strings.TrimSpace(c.Prompt)
		if c.Prompt == "" {
			return Config{}, fmt.Errorf("workgroup: copilot %q prompt is empty", c.Label)
		}
		if n := utf8.RuneCountInString(c.Prompt); n > MaxPromptRunes {
			return Config{}, fmt.Errorf("workgroup: copilot %q prompt is %d characters and the limit is %d", c.Label, n, MaxPromptRunes)
		}
	}
	return cfg, nil
}
