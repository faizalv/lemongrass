package usecase

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type fileEntry struct {
	name         string
	lines, chars int
}

func files(projectDir, pattern string) string {
	if pattern == "" {
		return "error: pattern required"
	}
	isGlob := strings.ContainsAny(pattern, "*?[")

	type rawMatch struct {
		rel          string
		lines, chars int
	}

	var allMatches []rawMatch
	_ = filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(projectDir, path)
		if err != nil {
			return nil
		}
		matched := false
		if isGlob {
			if ok, _ := filepath.Match(pattern, filepath.Base(rel)); ok {
				matched = true
			} else if ok, _ := filepath.Match(pattern, rel); ok {
				matched = true
			}
		} else {
			matched = strings.Contains(rel, pattern)
		}
		if !matched {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := bytes.Count(data, []byte("\n"))
		if len(data) > 0 && data[len(data)-1] != '\n' {
			lines++
		}
		allMatches = append(allMatches, rawMatch{rel: rel, lines: lines, chars: len(data)})
		return nil
	})
	if len(allMatches) == 0 {
		return "no results"
	}

	dirOrder := []string{}
	dirFiles := map[string][]fileEntry{}
	for _, m := range allMatches {
		dir := filepath.Dir(m.rel)
		if _, seen := dirFiles[dir]; !seen {
			dirOrder = append(dirOrder, dir)
		}
		dirFiles[dir] = append(dirFiles[dir], fileEntry{filepath.Base(m.rel), m.lines, m.chars})
	}

	// Find anchors: dirs with no ancestor in the result set.
	dirSet := map[string]bool{}
	for _, d := range dirOrder {
		dirSet[d] = true
	}
	var anchorOrder []string
	for _, d := range dirOrder {
		if findAncestor(d, dirSet) == "" {
			anchorOrder = append(anchorOrder, d)
		}
	}

	// Map each non-anchor dir to its deepest anchor ancestor.
	subDirs := map[string][]string{}
	for _, d := range dirOrder {
		if findAncestor(d, dirSet) == "" {
			continue // it is an anchor
		}
		anc := deepestAnchor(d, anchorOrder)
		if anc != "" {
			subDirs[anc] = append(subDirs[anc], d)
		}
	}
	for anc := range subDirs {
		sort.Strings(subDirs[anc])
	}

	var sb strings.Builder
	first := true

	for _, anchor := range anchorOrder {
		if !first {
			sb.WriteString("\n")
		}
		first = false

		label := anchor + "/"
		if anchor == "." {
			label = "./"
		}
		fmt.Fprintf(&sb, "%s\n", label)
		renderFileGroup(dirFiles[anchor], "  ", &sb)

		for _, sub := range subDirs[anchor] {
			rel, _ := filepath.Rel(anchor, sub)
			subEntries := dirFiles[sub]
			if len(subEntries) == 1 {
				e := subEntries[0]
				fmt.Fprintf(&sb, "  %s/%s  -- %s lines; %s chars\n", rel, e.name, formatCount(e.lines), formatCount(e.chars))
			} else {
				fmt.Fprintf(&sb, "  /%s/\n", rel)
				renderFileGroup(subEntries, "    ", &sb)
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// renderFileGroup writes entries to sb with optional prefix collapsing.
// Groups of 2+ consecutive files sharing a common prefix get a "prefix*" header.
func renderFileGroup(entries []fileEntry, indent string, sb *strings.Builder) {
	type pGroup struct {
		prefix  string
		entries []fileEntry
	}

	var groups []pGroup
	var pending []fileEntry

	flush := func() {
		if len(pending) > 0 {
			groups = append(groups, pGroup{"", append([]fileEntry(nil), pending...)})
			pending = pending[:0]
		}
	}

	i := 0
	for i < len(entries) {
		j := i + 1
		for j < len(entries) {
			p := trimToSep(lcPrefix(entries[i].name, entries[j].name))
			if len(p) >= 3 {
				j++
			} else {
				break
			}
		}
		if j > i+1 {
			cp := entries[i].name
			for _, e := range entries[i+1 : j] {
				cp = lcPrefix(cp, e.name)
			}
			cp = trimToSep(cp)
			if len(cp) >= 3 {
				flush()
				groups = append(groups, pGroup{cp, entries[i:j]})
				i = j
				continue
			}
		}
		pending = append(pending, entries[i])
		i++
	}
	flush()

	for _, g := range groups {
		if g.prefix != "" {
			fmt.Fprintf(sb, "%s%s*\n", indent, g.prefix)
			for _, e := range g.entries {
				fmt.Fprintf(sb, "%s  %s  -- %s lines; %s chars\n", indent, e.name[len(g.prefix):], formatCount(e.lines), formatCount(e.chars))
			}
		} else {
			for _, e := range g.entries {
				fmt.Fprintf(sb, "%s%s  -- %s lines; %s chars\n", indent, e.name, formatCount(e.lines), formatCount(e.chars))
			}
		}
	}
}

func findAncestor(dir string, dirSet map[string]bool) string {
	for candidate := range dirSet {
		if candidate == dir {
			continue
		}
		rel, err := filepath.Rel(candidate, dir)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		return candidate
	}
	return ""
}

func deepestAnchor(dir string, anchors []string) string {
	best := ""
	for _, a := range anchors {
		if a == dir {
			continue
		}
		rel, err := filepath.Rel(a, dir)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if len(a) > len(best) {
			best = a
		}
	}
	return best
}

func lcPrefix(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return a[:i]
}

func trimToSep(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '-' || s[i] == '_' || s[i] == '.' {
			return s[:i+1]
		}
	}
	return ""
}
