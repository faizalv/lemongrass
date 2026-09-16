package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookCommand `json:"hooks"`
}

type hookSpec struct {
	event   string
	matcher string
}

var lgrassHookSpecs = []hookSpec{
	{event: "SessionStart"},
	{event: "SessionEnd"},
	{event: "PreToolUse"}, // matches every tool
	{event: "PostToolUse"},
}

// Idempotent, and preserves every other key and hook group already in the file.
func ensureClaudeHooks() error {
	lgrassPath, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(lgrassPath); err == nil {
		lgrassPath = resolved
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	raw := map[string]json.RawMessage{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parsing %s: %w", settingsPath, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	hooks := map[string][]hookGroup{}
	if hooksRaw, ok := raw["hooks"]; ok {
		if err := json.Unmarshal(hooksRaw, &hooks); err != nil {
			return fmt.Errorf("parsing hooks in %s: %w", settingsPath, err)
		}
	}

	changed := false
	for _, spec := range lgrassHookSpecs {
		if idx, ok := findLgrassHook(hooks[spec.event], spec.event); ok {
			if hooks[spec.event][idx].Matcher != spec.matcher {
				hooks[spec.event][idx].Matcher = spec.matcher
				changed = true
			}
			continue
		}
		hooks[spec.event] = append(hooks[spec.event], hookGroup{
			Matcher: spec.matcher,
			Hooks:   []hookCommand{{Type: "command", Command: fmt.Sprintf("%s hook %s", lgrassPath, spec.event)}},
		})
		changed = true
	}
	if !changed {
		return nil
	}

	hooksJSON, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	raw["hooks"] = hooksJSON

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, out, 0o644)
}

// Matches by suffix, not the full command, since the installed binary's path isn't fixed.
func hasLgrassHook(groups []hookGroup, event string) bool {
	_, ok := findLgrassHook(groups, event)
	return ok
}

// findLgrassHook returns the index of this event's own lgrass-owned hook group.
func findLgrassHook(groups []hookGroup, event string) (int, bool) {
	want := " hook " + event
	for i, g := range groups {
		for _, h := range g.Hooks {
			if strings.HasSuffix(h.Command, want) {
				return i, true
			}
		}
	}
	return -1, false
}
