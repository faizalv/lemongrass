// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// toolboxLimit caps the items so inventory.md stays short enough to scan at a glance; a full toolbox refuses a new
// item rather than evicting one, so pruning stays a deliberate choice.
const toolboxLimit = 40

// toolboxAboutLimit keeps an inventory line to one sentence.
const toolboxAboutLimit = 200

var (
	toolboxSlug = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	toolboxTag  = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)
	toolboxLine = regexp.MustCompile(`^(\S+) \[([^\]]*)\] (.+)$`)
)

// The hooks refuse every direct write under biblio/toolbox/ (guard rule toolbox-write), so these commands are the only
// way a model changes it and the limit holds.
const toolboxUsage = `usage:
  lgrass toolbox add <slug> --about "<sentence>" [--tags a,b] <file>...
  lgrass toolbox update <slug> [--about "<sentence>"] [--tags a,b] [--drop <file>]... [<file>...]
  lgrass toolbox remove <slug>
  lgrass toolbox list`

type toolboxItem struct {
	slug  string
	tags  []string
	about string
}

func (it toolboxItem) line() string {
	return fmt.Sprintf("%s [%s] %s", it.slug, strings.Join(it.tags, ","), it.about)
}

type toolboxArgs struct {
	slug     string
	about    string
	hasAbout bool
	tags     []string
	hasTags  bool
	drops    []string
	files    []string
}

func cmdToolbox(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, toolboxUsage)
		os.Exit(1)
	}
	root := currentProject().Path
	var out string
	var err error
	switch args[0] {
	case "add":
		out, err = toolboxAdd(root, args[1:])
	case "update":
		out, err = toolboxUpdate(root, args[1:])
	case "remove":
		out, err = toolboxRemove(root, args[1:])
	case "list":
		out, err = toolboxList(root, args[1:])
	default:
		err = fmt.Errorf("lgrass: unknown toolbox command %q\n%s", args[0], toolboxUsage)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(out)
}

func toolboxDir(root string) string {
	return filepath.Join(root, "biblio", "toolbox")
}

func parseToolboxArgs(args []string, wantSlug bool) (toolboxArgs, error) {
	var a toolboxArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("lgrass: %s needs a value\n%s", arg, toolboxUsage)
			}
			i++
			return args[i], nil
		}
		switch arg {
		case "--about":
			v, err := value()
			if err != nil {
				return a, err
			}
			a.about, a.hasAbout = strings.TrimSpace(v), true
		case "--tags":
			v, err := value()
			if err != nil {
				return a, err
			}
			a.hasTags = true
			a.tags = nil
			for _, t := range strings.Split(v, ",") {
				if t = strings.TrimSpace(t); t != "" {
					a.tags = append(a.tags, t)
				}
			}
		case "--drop":
			v, err := value()
			if err != nil {
				return a, err
			}
			a.drops = append(a.drops, v)
		default:
			if strings.HasPrefix(arg, "--") {
				return a, fmt.Errorf("lgrass: unknown flag %s\n%s", arg, toolboxUsage)
			}
			if wantSlug && a.slug == "" {
				a.slug = arg
				continue
			}
			a.files = append(a.files, arg)
		}
	}
	if wantSlug && !toolboxSlug.MatchString(a.slug) {
		return a, fmt.Errorf("lgrass: %q is not a slug: lowercase letters and digits joined by hyphens\n%s", a.slug, toolboxUsage)
	}
	if a.hasAbout {
		if err := checkAbout(a.about); err != nil {
			return a, err
		}
	}
	for _, t := range a.tags {
		if !toolboxTag.MatchString(t) {
			return a, fmt.Errorf("lgrass: tag %q is not one token in lowercase or camelCase", t)
		}
	}
	return a, nil
}

func checkAbout(about string) error {
	switch {
	case about == "":
		return errors.New("lgrass: --about is empty; give one sentence on what the item is for")
	case strings.ContainsAny(about, "\r\n"):
		return errors.New("lgrass: --about must be one sentence on one line")
	case len(about) > toolboxAboutLimit:
		return fmt.Errorf("lgrass: --about is %d characters; keep it to one sentence under %d", len(about), toolboxAboutLimit)
	case !strings.HasSuffix(about, "."):
		return errors.New("lgrass: --about must be one sentence ending with a period")
	}
	return nil
}

func readInventory(root string) ([]toolboxItem, error) {
	data, err := os.ReadFile(filepath.Join(toolboxDir(root), "inventory.md"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lgrass: %w", err)
	}
	var items []toolboxItem
	for _, line := range strings.Split(string(data), "\n") {
		m := toolboxLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		var tags []string
		if m[2] != "" {
			tags = strings.Split(m[2], ",")
		}
		items = append(items, toolboxItem{slug: m[1], tags: tags, about: m[3]})
	}
	return items, nil
}

func writeInventory(root string, items []toolboxItem) error {
	sort.Slice(items, func(i, j int) bool { return items[i].slug < items[j].slug })
	var b strings.Builder
	for _, it := range items {
		b.WriteString(it.line() + "\n")
	}
	path := filepath.Join(toolboxDir(root), "inventory.md")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("lgrass: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("lgrass: %w", err)
	}
	return nil
}

func findItem(items []toolboxItem, slug string) int {
	for i, it := range items {
		if it.slug == slug {
			return i
		}
	}
	return -1
}

func requireBiblio(root string) error {
	if info, err := os.Stat(filepath.Join(root, "biblio")); err != nil || !info.IsDir() {
		return errors.New("lgrass: this project has no biblio/ yet; run `lgrass biblio init` first")
	}
	return nil
}

// copyIntoItem copies regular files into the item directory under their own names, replacing a file of the same name.
func copyIntoItem(itemDir string, files []string) ([]string, error) {
	var names []string
	for _, src := range files {
		info, err := os.Stat(src)
		if err != nil {
			return names, fmt.Errorf("lgrass: %w", err)
		}
		if !info.Mode().IsRegular() {
			return names, fmt.Errorf("lgrass: %s is not a regular file; an item holds files, one level deep", src)
		}
		name := filepath.Base(src)
		if err := copyFile(src, filepath.Join(itemDir, name), info.Mode().Perm()); err != nil {
			return names, err
		}
		names = append(names, name)
	}
	return names, nil
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("lgrass: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("lgrass: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("lgrass: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("lgrass: %w", err)
	}
	return nil
}

func toolboxAdd(root string, args []string) (string, error) {
	a, err := parseToolboxArgs(args, true)
	if err != nil {
		return "", err
	}
	if len(a.drops) > 0 {
		return "", fmt.Errorf("lgrass: --drop belongs to update\n%s", toolboxUsage)
	}
	if !a.hasAbout {
		return "", fmt.Errorf("lgrass: add needs --about, one sentence on what the item is for\n%s", toolboxUsage)
	}
	if len(a.files) == 0 {
		return "", fmt.Errorf("lgrass: add needs at least one file\n%s", toolboxUsage)
	}
	if err := requireBiblio(root); err != nil {
		return "", err
	}
	items, err := readInventory(root)
	if err != nil {
		return "", err
	}
	itemDir := filepath.Join(toolboxDir(root), a.slug)
	if findItem(items, a.slug) >= 0 {
		return "", fmt.Errorf("lgrass: the toolbox already holds %s; change it with `lgrass toolbox update %s`", a.slug, a.slug)
	}
	if _, err := os.Stat(itemDir); err == nil {
		return "", fmt.Errorf("lgrass: biblio/toolbox/%s exists but is not in inventory.md; pick another slug or ask the human about the folder", a.slug)
	}
	if len(items) >= toolboxLimit {
		return "", fmt.Errorf("lgrass: the toolbox holds %d items, its limit. Remove an item no longer used, or merge one into another with `lgrass toolbox update`, then add this one; `lgrass toolbox list` shows them", len(items))
	}
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		return "", fmt.Errorf("lgrass: %w", err)
	}
	names, err := copyIntoItem(itemDir, a.files)
	if err != nil {
		os.RemoveAll(itemDir)
		return "", err
	}
	items = append(items, toolboxItem{slug: a.slug, tags: a.tags, about: a.about})
	if err := writeInventory(root, items); err != nil {
		os.RemoveAll(itemDir)
		return "", err
	}
	return fmt.Sprintf("Added %s to the toolbox (%d of %d): %s\n", a.slug, len(items), toolboxLimit, strings.Join(names, ", ")), nil
}

func toolboxUpdate(root string, args []string) (string, error) {
	a, err := parseToolboxArgs(args, true)
	if err != nil {
		return "", err
	}
	if !a.hasAbout && !a.hasTags && len(a.drops) == 0 && len(a.files) == 0 {
		return "", fmt.Errorf("lgrass: update needs something to change\n%s", toolboxUsage)
	}
	if err := requireBiblio(root); err != nil {
		return "", err
	}
	items, err := readInventory(root)
	if err != nil {
		return "", err
	}
	i := findItem(items, a.slug)
	if i < 0 {
		return "", fmt.Errorf("lgrass: the toolbox holds no %s; `lgrass toolbox list` shows what it holds", a.slug)
	}
	itemDir := filepath.Join(toolboxDir(root), a.slug)
	for _, name := range a.drops {
		if name != filepath.Base(name) {
			return "", fmt.Errorf("lgrass: --drop takes a file name inside the item, not a path: %s", name)
		}
		if _, err := os.Stat(filepath.Join(itemDir, name)); err != nil {
			return "", fmt.Errorf("lgrass: %s holds no file %s", a.slug, name)
		}
	}
	var changes []string
	if len(a.files) > 0 {
		names, err := copyIntoItem(itemDir, a.files)
		if err != nil {
			return "", err
		}
		changes = append(changes, "copied "+strings.Join(names, ", "))
	}
	for _, name := range a.drops {
		if err := os.Remove(filepath.Join(itemDir, name)); err != nil {
			return "", fmt.Errorf("lgrass: %w", err)
		}
		changes = append(changes, "dropped "+name)
	}
	if entries, err := os.ReadDir(itemDir); err == nil && len(entries) == 0 {
		return "", fmt.Errorf("lgrass: %s now holds no file; add one with update, or remove the item with `lgrass toolbox remove %s`", a.slug, a.slug)
	}
	if a.hasAbout {
		items[i].about = a.about
		changes = append(changes, "new sentence")
	}
	if a.hasTags {
		items[i].tags = a.tags
		changes = append(changes, "new tags")
	}
	if err := writeInventory(root, items); err != nil {
		return "", err
	}
	return fmt.Sprintf("Updated %s: %s.\n", a.slug, strings.Join(changes, "; ")), nil
}

func toolboxRemove(root string, args []string) (string, error) {
	if len(args) != 1 || !toolboxSlug.MatchString(args[0]) {
		return "", fmt.Errorf("lgrass: remove takes one slug\n%s", toolboxUsage)
	}
	slug := args[0]
	if err := requireBiblio(root); err != nil {
		return "", err
	}
	items, err := readInventory(root)
	if err != nil {
		return "", err
	}
	i := findItem(items, slug)
	if i < 0 {
		return "", fmt.Errorf("lgrass: the toolbox holds no %s; `lgrass toolbox list` shows what it holds", slug)
	}
	if err := os.RemoveAll(filepath.Join(toolboxDir(root), slug)); err != nil {
		return "", fmt.Errorf("lgrass: %w", err)
	}
	items = append(items[:i], items[i+1:]...)
	if err := writeInventory(root, items); err != nil {
		return "", err
	}
	return fmt.Sprintf("Removed %s from the toolbox (%d of %d left). Its files stay in the biblio history.\n", slug, len(items), toolboxLimit), nil
}

func toolboxList(root string, args []string) (string, error) {
	if len(args) > 0 {
		return "", fmt.Errorf("lgrass: list takes no arguments\n%s", toolboxUsage)
	}
	if err := requireBiblio(root); err != nil {
		return "", err
	}
	items, err := readInventory(root)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return fmt.Sprintf("The toolbox is empty (limit %d). Add a reusable script or query with `lgrass toolbox add`.\n", toolboxLimit), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Toolbox, %d of %d items, in biblio/toolbox/<slug>/:\n", len(items), toolboxLimit)
	for _, it := range items {
		b.WriteString("  " + it.line() + "\n")
	}
	return b.String(), nil
}
