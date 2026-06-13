package usecase

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var blacklistedDirs = []string{"node_modules", "vendor", "dist", ".git", ".next", "__pycache__"}

func search(projectDir string, filePaths []string, args string) string {
	args = strings.TrimSpace(args)
	if args == "" {
		return "error: pattern required"
	}

	force := false
	if strings.HasSuffix(args, " --force") {
		force = true
		args = strings.TrimSuffix(args, " --force")
	}

	pattern := args
	var pathScope string
	if i := strings.LastIndex(pattern, " "); i >= 0 {
		last := pattern[i+1:]
		if strings.Contains(last, "/") {
			pathScope = last
			pattern = strings.TrimSpace(pattern[:i])
		}
	}

	if pathScope != "" && !force {
		clean := strings.Trim(pathScope, "/")
		first := strings.SplitN(clean, "/", 2)[0]
		for _, bl := range blacklistedDirs {
			if first == bl {
				return "[lg] " + bl + " is blacklisted -- append --force to search anyway"
			}
		}
	}

	if pathScope != "" {
		var scoped []string
		for _, p := range filePaths {
			if strings.HasPrefix(p, pathScope) {
				scoped = append(scoped, p)
			}
		}
		if len(scoped) == 0 {
			return fmt.Sprintf("no results for path %q -- if this is part of your pattern, remove the space or escape the slash", pathScope)
		}
		filePaths = scoped
	}

	var warns []string
	if len(pattern) >= 2 && pattern[0] == '"' && pattern[len(pattern)-1] == '"' {
		pattern = pattern[1 : len(pattern)-1]
		warns = append(warns, "quotes removed")
	}
	if strings.Contains(pattern, `\|`) {
		pattern = strings.ReplaceAll(pattern, `\|`, "|")
		warns = append(warns, `\| replaced with | for alternation`)
	}
	warnPrefix := ""
	if len(warns) > 0 {
		warnPrefix = "note: " + strings.Join(warns, "; ") + " -- use bare patterns with | for alternation, no quotes\n\n"
	}

	re, _ := regexp.Compile("(?i)" + pattern)
	matchLine := func(line string) bool {
		if re != nil {
			return re.MatchString(line)
		}
		return strings.Contains(strings.ToLower(line), strings.ToLower(pattern))
	}

	const contextLines = 2
	const maxResults = 50

	type match struct {
		filePath  string
		lineStart int
		lineEnd   int
		content   string
	}

	var results []match
	limitReached := false

	dirSeen := make(map[string]bool)
	for _, p := range filePaths {
		if matchLine(p) {
			results = append(results, match{filePath: p, content: "[path]"})
			if len(results) >= maxResults {
				limitReached = true
				break
			}
		}
		for d := filepath.Dir(p); d != "." && d != ""; d = filepath.Dir(d) {
			if dirSeen[d] {
				break
			}
			dirSeen[d] = true
		}
	}
	if !limitReached {
		dirs := make([]string, 0, len(dirSeen))
		for d := range dirSeen {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		for _, d := range dirs {
			if matchLine(d) {
				results = append(results, match{filePath: d + "/", content: "[dir]"})
				if len(results) >= maxResults {
					limitReached = true
					break
				}
			}
		}
	}

	if !limitReached {
		for _, relPath := range filePaths {
			if limitReached {
				break
			}
			absPath := filepath.Join(projectDir, relPath)
			data, err := os.ReadFile(absPath)
			if err != nil {
				continue
			}
			if bytes.IndexByte(data, 0) >= 0 {
				continue
			}
			fileLines := strings.Split(string(data), "\n")
			for i, line := range fileLines {
				if matchLine(line) {
					start := i - contextLines
					if start < 0 {
						start = 0
					}
					end := i + contextLines
					if end >= len(fileLines) {
						end = len(fileLines) - 1
					}
					results = append(results, match{
						filePath:  relPath,
						lineStart: start + 1,
						lineEnd:   end + 1,
						content:   strings.Join(fileLines[start:end+1], "\n"),
					})
					if len(results) >= maxResults {
						limitReached = true
						break
					}
				}
			}
		}
	}

	if len(results) == 0 {
		return warnPrefix + "no results"
	}

	var sb strings.Builder
	for i, r := range results {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		if r.lineStart == 0 {
			fmt.Fprintf(&sb, "%s  %s", r.filePath, r.content)
		} else {
			fmt.Fprintf(&sb, "%s L%d-%d\n%s", r.filePath, r.lineStart, r.lineEnd, r.content)
		}
	}
	if limitReached {
		fmt.Fprintf(&sb, "\n\n!! capped at %d results -- use a more specific pattern", maxResults)
	}
	return warnPrefix + sb.String()
}
