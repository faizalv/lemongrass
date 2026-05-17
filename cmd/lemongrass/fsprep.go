package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/faizalv/lemongrass/config"
)

var defaultFsExclusions = map[string]bool{
	// system virtual filesystems — can hang or error on walk
	"proc": true,
	"sys":  true,
	"dev":  true,
	// noisy system dirs
	"run":        true,
	"tmp":        true,
	"boot":       true,
	"lost+found": true,
	// project noise
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	"dist":         true,
	"build":        true,
	"target":       true,
	".next":        true,
	".nuxt":        true,
	"coverage":     true,
}

func walkFS(w io.Writer) {
	cfg := config.LoadOrDefault()
	excluded := make(map[string]bool, len(defaultFsExclusions)+len(cfg.FsExtraExclude))
	for k := range defaultFsExclusions {
		excluded[k] = true
	}
	for _, e := range cfg.FsExtraExclude {
		excluded[e] = true
	}

	visited := make(map[string]struct{})
	walkDir("/", excluded, visited, w)
}

func walkDir(path string, excluded map[string]bool, visited map[string]struct{}, w io.Writer) {
	reald, err := filepath.EvalSymlinks(path)
	if err != nil {
		return
	}
	if _, seen := visited[reald]; seen {
		return
	}
	visited[reald] = struct{}{}

	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if excluded[entry.Name()] {
			continue
		}
		full := filepath.Join(path, entry.Name())
		info, err := os.Stat(full) // follows symlinks
		if err != nil || !info.IsDir() {
			continue
		}
		fmt.Fprintln(w, full)
		walkDir(full, excluded, visited, w)
	}
}

func cmdFsPrep() {
	walkFS(os.Stdout)
}
