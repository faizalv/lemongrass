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

func (k *keeper) watchedDirs() []string {
	dirs := []string{
		filepath.Join(k.home, ".claude"),
		filepath.Join(k.home, ".claude", "skills"),
		filepath.Dir(k.claudeSkillPath()),
		filepath.Join(k.home, ".codex"),
		filepath.Join(k.home, ".codex", "skills"),
		filepath.Dir(k.codexSkillPath()),
	}
	for _, c := range k.candidates {
		dirs = append(dirs, filepath.Dir(c))
	}
	return dirs
}

func (k *keeper) relevant() map[string]bool {
	set := map[string]bool{
		k.claudeSettingsPath():                     true,
		filepath.Join(k.home, ".claude", "skills"): true,
		filepath.Dir(k.claudeSkillPath()):          true,
		k.claudeSkillPath():                        true,
		k.codexHooksPath():                         true,
		filepath.Join(k.home, ".codex", "skills"):  true,
		filepath.Dir(k.codexSkillPath()):           true,
		k.codexSkillPath():                         true,
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
