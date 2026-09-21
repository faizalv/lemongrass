package main

import (
	_ "embed"
	"errors"
	"os"
	"path/filepath"
)

//go:embed SKILL.md
var skillContent []byte

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

func (k *keeper) settingsPath() string {
	return filepath.Join(k.home, ".claude", "settings.json")
}

func (k *keeper) skillPath() string {
	return filepath.Join(k.home, ".claude", "skills", "lemongrass", "SKILL.md")
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
	if err := syncFile(k.skillPath(), skillContent); err != nil {
		errs = append(errs, err)
	}
	if path := k.lgrassPath(); path != "" {
		if err := syncSettings(k.settingsPath(), path); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
