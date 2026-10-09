package guard

import (
	"path/filepath"
	"strings"
)

var deletionScopeRules = map[string]bool{"rm-force": true, "rm-plain": true, "find-delete": true}

var protectedProjectDirs = []string{".git", "biblio"}

var directoryChangers = map[string]bool{"cd": true, "pushd": true, "popd": true}

// deletionScopeOpen reports whether deletions in the script may be judged by where their targets land in the project.
func (c checker) deletionScopeOpen(script shellScript) bool {
	root := c.projectRoot()
	if root == "" || c.in.Cwd == "" || !withinOrEqual(resolveFully(c.in.Cwd), root) {
		return false
	}
	for _, call := range script.Calls {
		if directoryChangers[call.Name] {
			return false
		}
	}
	return true
}

// deletesInProject reports whether an rm or find -delete call only removes paths inside the project root and outside its protected directories.
func (c checker) deletesInProject(call shellCall) bool {
	if call.Indirect {
		return false
	}
	switch call.Name {
	case "rm":
		if hasLongFlag(call.Args, "--no-preserve-root") {
			return false
		}
		return c.allInProject(rmTargets(call.Args))
	case "find":
		roots, followsLinks := findRoots(call.Args)
		return !followsLinks && c.allInProject(roots)
	}
	return false
}

func (c checker) allInProject(targets []string) bool {
	root := c.projectRoot()
	if len(targets) == 0 {
		return false
	}
	for _, target := range targets {
		if !literalTarget(target) {
			return false
		}
		resolved := []string{ResolvePath(target, c.in.Cwd, c.in.Home)}
		if strings.ContainsAny(target, "*?[") {
			matches, err := filepath.Glob(resolved[0])
			if err != nil {
				return false
			}
			resolved = append(resolved, matches...)
		}
		for _, p := range resolved {
			location := filepath.Join(resolveExisting(filepath.Dir(p)), filepath.Base(p))
			for _, candidate := range []string{location, resolveFully(p)} {
				if !strictlyWithin(candidate, root) || isProtected(candidate, root) {
					return false
				}
			}
		}
	}
	return true
}

func (c checker) projectRoot() string {
	if c.in.ProjectRoot == "" {
		return ""
	}
	return resolveFully(c.in.ProjectRoot)
}

func rmTargets(args []string) []string {
	var targets []string
	flagsDone := false
	for _, arg := range args {
		if !flagsDone && arg == "--" {
			flagsDone = true
			continue
		}
		if !flagsDone && strings.HasPrefix(arg, "-") && len(arg) > 1 {
			continue
		}
		targets = append(targets, arg)
	}
	return targets
}

func findRoots(args []string) (roots []string, followsLinks bool) {
	i := 0
	for ; i < len(args); i++ {
		arg := args[i]
		if arg == "-L" || arg == "-H" {
			followsLinks = true
		} else if arg == "-D" {
			i++
		} else if arg != "-P" && !strings.HasPrefix(arg, "-O") {
			break
		}
	}
	for ; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") || arg == "(" || arg == "!" || arg == "," || arg == ")" {
			break
		}
		roots = append(roots, arg)
	}
	if containsArg(args, "-follow") {
		followsLinks = true
	}
	if len(roots) == 0 {
		roots = []string{"."}
	}
	return roots, followsLinks
}

func literalTarget(target string) bool {
	if strings.HasPrefix(target, "~") && target != "~" && !strings.HasPrefix(target, "~/") {
		return false
	}
	rest := target
	for _, prefix := range []string{"$HOME", "${HOME}"} {
		if strings.HasPrefix(rest, prefix) {
			rest = strings.TrimPrefix(rest, prefix)
			break
		}
	}
	return !strings.ContainsAny(rest, "${}`")
}

func isProtected(p, root string) bool {
	scratchpad := filepath.Join(root, "biblio", "scratchpad")
	if strictlyWithin(p, scratchpad) && !withinOrEqual(p, filepath.Join(scratchpad, "archive")) {
		return false
	}
	for _, dir := range protectedProjectDirs {
		if withinOrEqual(p, filepath.Join(root, dir)) {
			return true
		}
	}
	return false
}

func resolveFully(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Clean(resolved)
	}
	return resolveExisting(p)
}

// resolveExisting resolves symlinks in the deepest existing ancestor of p and keeps the missing remainder as written.
func resolveExisting(p string) string {
	dir, rest := filepath.Clean(p), ""
	for {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Clean(p)
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
}

func strictlyWithin(p, root string) bool {
	return strings.HasPrefix(p, strings.TrimRight(root, string(filepath.Separator))+string(filepath.Separator))
}

func withinOrEqual(p, root string) bool {
	return p == root || strictlyWithin(p, root)
}
