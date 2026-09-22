package main

import (
	_ "embed"
	"errors"
	"os"
	"path/filepath"
)

//go:embed SKILL.md
var claudeSkillContent []byte

//go:embed CODEX_SKILL.md
var codexSkillContent []byte

type keeper struct {
	home       string
	candidates []string
}

func newKeeper(home string) *keeper {
	return &keeper{
		home: home,
		candidates: []string{
			filepath.Join(home, ".local", "bin", "lgrass"),
			"/usr/local/bin/lgrass",
		},
	}
}

func (k *keeper) claudeSettingsPath() string {
	return filepath.Join(k.home, ".claude", "settings.json")
}

func (k *keeper) claudeSkillPath() string {
	return filepath.Join(k.home, ".claude", "skills", "lemongrass", "SKILL.md")
}

func (k *keeper) codexHooksPath() string {
	return filepath.Join(k.home, ".codex", "hooks.json")
}

func (k *keeper) codexSkillPath() string {
	return filepath.Join(k.home, ".codex", "skills", "lemongrass", "SKILL.md")
}

func (k *keeper) lgrassPath() string {
	for _, c := range k.candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

func (k *keeper) reconcile() error {
	var errs []error
	if err := syncFile(k.claudeSkillPath(), claudeSkillContent); err != nil {
		errs = append(errs, err)
	}
	if err := syncFile(k.codexSkillPath(), codexSkillContent); err != nil {
		errs = append(errs, err)
	}
	if path := k.lgrassPath(); path != "" {
		if err := syncHookConfig(k.claudeSettingsPath(), path, reconcileClaudeSettings); err != nil {
			errs = append(errs, err)
		}
		if err := syncHookConfig(k.codexHooksPath(), path, reconcileCodexHooks); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
