// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"path/filepath"
	"strings"
)

// TouchedPaths lists the places a call names literally, resolved: its cwd, its tool paths, and for a shell command
// every cd or pushd target and every path-like argument and redirect, each resolved against every directory the
// command could run from. It reads only literal text, so a path built from variables or reached by a script is
// missing; callers use it to find which projects a call reaches, never to prove a call stays inside one.
func TouchedPaths(in Input) []string {
	home := ResolveDir(in.Home)
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	cwd := ""
	if in.Cwd != "" {
		cwd = resolveFully(in.Cwd)
		add(cwd)
	}
	for _, p := range in.Paths {
		add(ResolvePath(p, cwd, home))
	}
	var script shellScript
	var err error
	switch in.Tool {
	case "Bash":
		script, err = parseShell(in.Command)
	case "PowerShell":
		err = script.addPowerShell(in.Command, 0)
	default:
		return out
	}
	if err != nil || strings.TrimSpace(in.Command) == "" {
		return out
	}
	dirs := commandDirs(script, cwd, home)
	for _, d := range dirs {
		add(d)
	}
	var named []string
	for _, call := range script.Calls {
		if call.Dynamic {
			continue
		}
		if pathLike(call.Path) {
			named = append(named, call.Path)
		}
		for i, arg := range call.Args {
			if i < len(call.ArgDynamic) && call.ArgDynamic[i] {
				continue
			}
			if strings.HasPrefix(arg, "-") {
				_, value, ok := strings.Cut(arg, "=")
				if !ok {
					continue
				}
				arg = value
			}
			if pathLike(arg) && literalTarget(arg) {
				named = append(named, arg)
			}
		}
	}
	for _, r := range script.Redirects {
		if literalTarget(r.Path) {
			named = append(named, r.Path)
		}
	}
	for _, p := range named {
		for _, d := range dirs {
			add(ResolvePath(p, d, home))
		}
	}
	return out
}

// commandDirs is the cwd plus every literal cd or pushd target resolved against each directory collected before
// it, the same over-approximation the deletion scope uses, without its bound to one project.
func commandDirs(script shellScript, cwd, home string) []string {
	var dirs []string
	if cwd != "" {
		dirs = append(dirs, cwd)
	}
	for _, call := range script.Calls {
		if call.Name != "cd" && call.Name != "pushd" {
			continue
		}
		target, ok := directoryTarget(call)
		if !ok || !literalTarget(target) {
			continue
		}
		bases := dirs
		if len(bases) == 0 {
			bases = []string{""}
		}
		for _, base := range bases {
			if base == "" && !filepath.IsAbs(target) && !strings.HasPrefix(target, "~") {
				continue
			}
			next := resolveFully(ResolvePath(target, base, home))
			if !containsArg(dirs, next) {
				dirs = append(dirs, next)
			}
		}
	}
	return dirs
}

// pathLike keeps the arguments that can name a place outside the cwd; a bare word resolves inside a directory the
// set already holds.
func pathLike(arg string) bool {
	return arg != "" && (filepath.IsAbs(arg) || strings.HasPrefix(arg, "~") || strings.ContainsAny(arg, `/\`))
}
