// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package how

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	numberedRow = regexp.MustCompile(`^(\d+)\.\s+(\S+)\s.*->\s*(\S+)\s*$`)
	tocLine     = regexp.MustCompile(`^(\S+) \[([^\]]*)\]( warning: .+)?$`)
	tagToken    = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	snakeCase   = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)
)

// keeperSweep finds the structural drift itself and prints one line per finding, so its output grows with the drift, never with the size of the project.
func keeperSweep(root string, args []string) (string, error) {
	if len(args) > 0 {
		return "", fmt.Errorf("how: keeper sweep takes no arguments")
	}
	biblio := filepath.Join(root, "biblio")
	if info, err := os.Stat(biblio); err != nil || !info.IsDir() {
		return "", fmt.Errorf("how: no biblio/ directory in %s", root)
	}
	var findings []string
	findings = append(findings, sweepHandovers(biblio)...)
	findings = append(findings, sweepScratchpad(root, biblio)...)
	findings = append(findings, sweepBooks(biblio)...)

	var b strings.Builder
	b.WriteString(fragment("keeper/rules.md", nil))
	if len(findings) == 0 {
		b.WriteString("\n\n## Structural drift\n\nNone found in the whiteboard, handovers, scratchpad, toc or names.\n")
	} else {
		fmt.Fprintf(&b, "\n\n## Structural drift, %d finding(s)\n\nFix each in place. A rename propagates to the toc and every reference.\n\n", len(findings))
		for _, f := range findings {
			b.WriteString("- " + f + "\n")
		}
	}
	b.WriteString("\n## Judgment\n\n" + fragment("keeper/judgment.md", nil) + "\n\n" + fragment("keeper/report.md", nil) + "\n")
	return b.String(), nil
}

func sweepHandovers(biblio string) []string {
	dir := filepath.Join(biblio, "handover")
	var out []string
	files := map[string]bool{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" && e.Name() != "whiteboard.md" {
			files[e.Name()] = true
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "whiteboard.md"))
	if err != nil && len(files) > 0 {
		out = append(out, "biblio/handover/whiteboard.md is missing while handovers exist: create it with one row per handover.")
	}
	rows := map[string]bool{}
	want := 1
	numbered := true
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := numberedRow.FindStringSubmatch(line)
		if m == nil {
			out = append(out, fmt.Sprintf("whiteboard line %q is not in the form `N. slug [tags] -> file.md`.", line))
			continue
		}
		if n, _ := strconv.Atoi(m[1]); n != want {
			numbered = false
		}
		want++
		file := m[3]
		rows[file] = true
		if !files[file] {
			if _, err := os.Stat(filepath.Join(dir, "archive", file)); err == nil {
				out = append(out, fmt.Sprintf("whiteboard row %s points at an archived handover: drop the row.", m[2]))
			} else {
				out = append(out, fmt.Sprintf("whiteboard row %s points at %s, which does not exist: drop the row or restore the handover.", m[2], file))
			}
		}
	}
	if !numbered {
		out = append(out, "whiteboard rows are not numbered 1, 2, 3 in order: renumber in place.")
	}
	for _, name := range sortedKeys(files) {
		if !rows[name] {
			out = append(out, fmt.Sprintf("biblio/handover/%s has no whiteboard row: add one at its priority.", name))
		}
		out = append(out, handoverShape(biblio, name)...)
	}
	return out
}

var prdRef = regexp.MustCompile(`scratchpad/([^/\s` + "`" + `]+)/prd\.md`)

func handoverShape(biblio, name string) []string {
	data, err := os.ReadFile(filepath.Join(biblio, "handover", name))
	if err != nil {
		return nil
	}
	text := string(data)
	var out []string
	if !regexp.MustCompile(`(?m)^## Status`).MatchString(text) {
		out = append(out, fmt.Sprintf("biblio/handover/%s has no Status section.", name))
	}
	m := prdRef.FindStringSubmatch(text)
	switch {
	case m == nil:
		out = append(out, fmt.Sprintf("biblio/handover/%s points at no prd.md: a handover points at its task's PRD.", name))
	case !exists(filepath.Join(biblio, "scratchpad", m[1], "prd.md")):
		out = append(out, fmt.Sprintf("biblio/handover/%s points at scratchpad/%s/prd.md, which is not an active task.", name, m[1]))
	}
	return out
}

func sweepScratchpad(root, biblio string) []string {
	entries, err := os.ReadDir(filepath.Join(biblio, "scratchpad"))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.Name() == "archive" || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if !e.IsDir() {
			out = append(out, fmt.Sprintf("biblio/scratchpad/%s is a bare file: move it into a task directory.", e.Name()))
			continue
		}
		f := inspectWork(root, e.Name())
		if len(f.files) == 0 {
			out = append(out, fmt.Sprintf("scratchpad/%s holds no prd.md, recon.md, debug.md or notes.md: report it, it fits no task type.", e.Name()))
		}
		if !exists(filepath.Join(biblio, "scratchpad", e.Name(), "activities.md")) {
			out = append(out, fmt.Sprintf("scratchpad/%s has no activities.md.", e.Name()))
		}
		if f.has("prd.md") {
			if plans, _, open := readPRD(filepath.Join(biblio, "scratchpad", e.Name(), "prd.md")); plans > 0 && len(open) == 0 {
				out = append(out, fmt.Sprintf("scratchpad/%s has every Plans item done: report it as ready for `lgrass how task close %s`.", e.Name(), e.Name()))
			}
		}
	}
	return out
}

func sweepBooks(biblio string) []string {
	dir := filepath.Join(biblio, "books")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	books := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() {
			if e.Name() != "toc.md" {
				out = append(out, fmt.Sprintf("biblio/books/%s is a file outside any book: move it into a book folder.", e.Name()))
			}
			continue
		}
		books[e.Name()] = true
		if !snakeCase.MatchString(e.Name()) {
			out = append(out, fmt.Sprintf("book folder %s is not a plain snake_case title: rename it, keeping its tags on the toc line.", e.Name()))
		}
		chapters, _ := os.ReadDir(filepath.Join(dir, e.Name()))
		for _, c := range chapters {
			if c.IsDir() || filepath.Ext(c.Name()) != ".md" || !snakeCase.MatchString(strings.TrimSuffix(c.Name(), ".md")) {
				out = append(out, fmt.Sprintf("%s/%s is not a snake_case .md chapter: rename it.", e.Name(), c.Name()))
			}
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "toc.md"))
	if err != nil {
		if len(books) > 0 {
			out = append(out, "biblio/books/toc.md is missing: create it with one line per book.")
		}
		return out
	}
	listed := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := tocLine.FindStringSubmatch(line)
		if m == nil {
			out = append(out, fmt.Sprintf("toc line %q is not `folder [tags]` with an optional `warning:`: remove the description.", clip(line)))
			continue
		}
		listed[m[1]] = true
		if !books[m[1]] {
			out = append(out, fmt.Sprintf("toc lists %s, which has no book folder: drop the line.", m[1]))
		}
		var bad []string
		for _, tag := range strings.Split(m[2], ",") {
			if !tagToken.MatchString(strings.TrimSpace(tag)) {
				bad = append(bad, strings.TrimSpace(tag))
			}
		}
		if len(bad) > 0 {
			out = append(out, fmt.Sprintf("toc line %s has tags that are not one lowercase or camelCase token: %s.", m[1], strings.Join(bad, ", ")))
		}
	}
	var missing []string
	for name := range books {
		if !listed[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	for _, name := range missing {
		out = append(out, fmt.Sprintf("book %s has no toc line: add one with its tags.", name))
	}
	return out
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func clip(s string) string {
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}
