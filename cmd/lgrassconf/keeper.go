// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/faizalv/lemongrass/internal/agentconf"
	"github.com/faizalv/lemongrass/internal/claudeaccount"
	"github.com/faizalv/lemongrass/internal/config"
	"github.com/faizalv/lemongrass/internal/session"
)

var legacySkillNames = []string{"lemongrass", "lgrass-staleness", "lgrass-closing", "lgrass-copilot", "lgrass-pilot", "lgrass-connector"}

// No lemongrass skill is installed for any vendor anymore; the keeper only removes old copies from each vendor's skills folder.
type vendor struct {
	configDir string
	// retired are lemongrass skills replaced by lgrass how, removed like the legacy ones.
	retired []string
}

var (
	claude  = vendor{configDir: ".claude", retired: retiredSkills()}
	codex   = vendor{configDir: ".codex", retired: retiredSkills()}
	vendors = []vendor{claude, codex}
)

func retiredSkills() []string {
	names := make([]string, 0, len(session.RetiredSkills))
	for name := range session.RetiredSkills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// removedSkills are the skill directories the keeper deletes for this vendor.
func (v vendor) removedSkills() []string {
	return append(append([]string{}, legacySkillNames...), v.retired...)
}

type skillFile struct {
	root    string
	path    string
	content []byte
}

type keeper struct {
	home        string
	candidates  []string
	orientation []skillFile
	plugin      []skillFile
	// onPass, when set, runs as each watcher pass starts (finished false) and once it has reconciled and armed its watches (finished true). Tests use it to change a file only when no pass is in flight, since a pass writes files before addWatches arms them and kqueue arms a directory watch slowly; nil in production.
	onPass func(finished bool)
}

func newKeeper(home string) *keeper {
	k := &keeper{
		home:       home,
		candidates: []string{config.LgrassdPath(home)},
	}
	k.orientation = k.loadOrientation()
	k.plugin = k.loadPlugin()
	return k
}

func (k *keeper) skillsRoot(v vendor) string {
	return filepath.Join(k.home, v.configDir, "skills")
}

func (k *keeper) legacySkillDir(v vendor, name string) string {
	return filepath.Join(k.skillsRoot(v), name)
}

func (k *keeper) accounts() claudeaccount.Accounts {
	return claudeaccount.Accounts{Home: k.home, HookBinary: k.lgrassdPath()}
}

func (k *keeper) claudeSettingsPath() string {
	return filepath.Join(k.home, ".claude", "settings.json")
}

func (k *keeper) codexHooksPath() string {
	return filepath.Join(k.home, ".codex", "hooks.json")
}

func (k *keeper) agyHooksPath() string {
	return filepath.Join(k.home, ".gemini", "config", "hooks.json")
}

func (k *keeper) lgrassdPath() string {
	for _, c := range k.candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

func removeLegacySkill(dir string) error {
	if err := os.Remove(filepath.Join(dir, "SKILL.md")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	return os.Remove(dir)
}

func (k *keeper) reconcile() error {
	var errs []error
	for _, f := range k.managedFiles() {
		if err := agentconf.SyncFile(f.path, f.content); err != nil {
			errs = append(errs, err)
		}
	}
	for _, v := range vendors {
		for _, name := range v.removedSkills() {
			if err := removeLegacySkill(k.legacySkillDir(v, name)); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if path := k.lgrassdPath(); path != "" {
		if err := agentconf.SyncHookConfig(k.claudeSettingsPath(), path, agentconf.ReconcileClaudeSettings); err != nil {
			errs = append(errs, err)
		}
		if err := agentconf.SyncHookConfig(k.codexHooksPath(), path, reconcileCodexHooks); err != nil {
			errs = append(errs, err)
		}
		if err := agentconf.SyncHookConfig(k.agyHooksPath(), path, reconcileAgyHooks); err != nil {
			errs = append(errs, err)
		}
	}
	accounts := k.accounts()
	if err := agentconf.SyncStatusLine(k.claudeSettingsPath(), agentconf.StatusLineBinary(k.home, k.lgrassdPath()), "primary", accounts.StatusLines()); err != nil {
		errs = append(errs, err)
	}
	if err := accounts.Reconcile(); err != nil {
		errs = append(errs, err)
	}
	if names, err := accounts.Names(); err == nil {
		if err := accounts.StatusLines().Prune(append([]string{"primary"}, names...)); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
