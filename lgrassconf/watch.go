package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	debounce      = 200 * time.Millisecond
	safetyNetTick = 10 * time.Minute
)

func (k *keeper) skillDirs() []string {
	seen := map[string]bool{}
	var dirs []string
	add := func(dir string) {
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	for _, v := range vendors {
		add(filepath.Join(k.home, v.configDir))
		add(k.skillsRoot(v))
		add(k.legacySkillDir(v))
	}
	for _, f := range k.skills {
		for dir := filepath.Dir(f.path); dir != f.root; dir = filepath.Dir(dir) {
			add(dir)
		}
	}
	return dirs
}

func (k *keeper) watchedDirs() []string {
	dirs := k.skillDirs()
	for _, c := range k.candidates {
		dirs = append(dirs, filepath.Dir(c))
	}
	return dirs
}

func (k *keeper) relevant() map[string]bool {
	set := map[string]bool{
		k.claudeSettingsPath(): true,
		k.codexHooksPath():     true,
	}
	for _, dir := range k.skillDirs() {
		set[dir] = true
	}
	for _, f := range k.skills {
		set[f.path] = true
	}
	for _, v := range vendors {
		set[filepath.Join(k.legacySkillDir(v), "SKILL.md")] = true
	}
	for _, c := range k.candidates {
		set[c] = true
	}
	return set
}

func (k *keeper) addWatches(w *fsnotify.Watcher) {
	for _, dir := range k.watchedDirs() {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			w.Add(dir)
		}
	}
}

func (k *keeper) run(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	pass := func() {
		if err := k.reconcile(); err != nil {
			log.Printf("reconcile: %v", err)
		}
		k.addWatches(w)
	}
	pass()

	relevant := k.relevant()
	timer := time.NewTimer(debounce)
	timer.Stop()
	tick := time.NewTicker(safetyNetTick)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-w.Events:
			if relevant[ev.Name] {
				timer.Reset(debounce)
			}
		case err := <-w.Errors:
			log.Printf("watcher: %v", err)
			timer.Reset(debounce)
		case <-timer.C:
			pass()
		case <-tick.C:
			pass()
		}
	}
}
