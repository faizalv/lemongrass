package main

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed skills
var skillsFS embed.FS

const legacySkillName = "lemongrass"

type vendor struct {
	embedRoot string
	configDir string
}

var (
	claude  = vendor{embedRoot: "skills/claude", configDir: ".claude"}
	codex   = vendor{embedRoot: "skills/codex", configDir: ".codex"}
	vendors = []vendor{claude, codex}
)

type skillFile struct {
	root    string
	path    string
	content []byte
}

type keeper struct {
	home       string
	candidates []string
	skills     []skillFile
}

func newKeeper(home string) *keeper {
	k := &keeper{
		home: home,
		candidates: []string{
			filepath.Join(home, ".local", "bin", "lgrass"),
			"/usr/local/bin/lgrass",
		},
	}
	k.skills = k.loadSkills()
	return k
}

func (k *keeper) loadSkills() []skillFile {
	var files []skillFile
	for _, v := range vendors {
		root := k.skillsRoot(v)
		err := fs.WalkDir(skillsFS, v.embedRoot, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := skillsFS.ReadFile(p)
			if err != nil {
				return err
			}
			rel := strings.TrimPrefix(p, v.embedRoot+"/")
			files = append(files, skillFile{root: root, path: filepath.Join(root, filepath.FromSlash(rel)), content: data})
			return nil
		})
		if err != nil {
			panic(err)
		}
	}
	return files
}

func (k *keeper) skillsRoot(v vendor) string {
	return filepath.Join(k.home, v.configDir, "skills")
}

func (k *keeper) legacySkillDir(v vendor) string {
	return filepath.Join(k.skillsRoot(v), legacySkillName)
}

func (k *keeper) claudeSettingsPath() string {
	return filepath.Join(k.home, ".claude", "settings.json")
}

func (k *keeper) codexHooksPath() string {
	return filepath.Join(k.home, ".codex", "hooks.json")
}

func (k *keeper) lgrassPath() string {
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
	for _, f := range k.skills {
		if err := syncFile(f.path, f.content); err != nil {
			errs = append(errs, err)
		}
	}
	for _, v := range vendors {
		if err := removeLegacySkill(k.legacySkillDir(v)); err != nil {
			errs = append(errs, err)
		}
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
