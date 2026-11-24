// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The Biblio pane's "Set up biblio" button writes the same layout from the Electron main process
// (ui/src/main/biblio.ts, biblio:init), so a change here is made there too.
var biblioInitDirs = []string{"scratchpad", "laws", "handover", "books", "toolbox"}

// The index files start empty: the session hook delivers no laws block for an empty summary.md, and keeper sweep
// accepts an empty toc and whiteboard.
var biblioInitFiles = []string{"books/toc.md", "laws/summary.md", "handover/whiteboard.md", "toolbox/inventory.md"}

func cmdBiblioInit(args []string) {
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrass biblio init")
		os.Exit(1)
	}
	created, err := biblioInit(currentProject().Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(created) == 0 {
		fmt.Println("biblio/ is already set up; nothing changed.")
		return
	}
	fmt.Println("Set up biblio/:")
	for _, c := range created {
		fmt.Println("  " + c)
	}
	fmt.Println("\nContinue with the `lgrass how work` action that fits the prompt.")
}

// biblioInit creates what is missing of biblio/'s layout under root and lists it, leaving every existing file as it
// is, so running it again changes nothing.
func biblioInit(root string) ([]string, error) {
	biblio := filepath.Join(root, "biblio")
	if info, err := os.Lstat(biblio); err == nil && !info.IsDir() {
		return nil, fmt.Errorf("lgrass: %s exists and is not a folder", biblio)
	}
	var created []string
	for _, d := range biblioInitDirs {
		p := filepath.Join(biblio, d)
		if _, err := os.Stat(p); err == nil {
			continue
		}
		if err := os.MkdirAll(p, 0o755); err != nil {
			return created, fmt.Errorf("lgrass: %w", err)
		}
		created = append(created, "biblio/"+d+"/")
	}
	for _, f := range biblioInitFiles {
		p := filepath.Join(biblio, filepath.FromSlash(f))
		file, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return created, fmt.Errorf("lgrass: %w", err)
		}
		file.Close()
		created = append(created, "biblio/"+f)
	}
	added, err := ignoreBiblio(root)
	if err != nil {
		return created, err
	}
	if added {
		created = append(created, "biblio/ in .gitignore")
	}
	return created, nil
}

// ignoreBiblio appends biblio/ to the project's .gitignore unless a line already names it at the root, because
// biblio/ is each person's own working knowledge and is never committed.
func ignoreBiblio(root string) (bool, error) {
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("lgrass: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		switch strings.TrimSpace(line) {
		case "biblio", "biblio/", "/biblio", "/biblio/":
			return false, nil
		}
	}
	text := string(data)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if err := os.WriteFile(path, []byte(text+"biblio/\n"), 0o644); err != nil {
		return false, fmt.Errorf("lgrass: %w", err)
	}
	return true, nil
}
