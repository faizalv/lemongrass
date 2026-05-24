package usecase

import (
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"

	"github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang"
)

// neverIgnore lists paths that cannot be un-ignored with !
var neverIgnore = []string{".git", ".lgignore"}

var defaultPatterns = []string{
	"node_modules/",
	"vendor/",
	"dist/",
	"build/",
	"bin/",
	".cache/",
	".next/",
	".nuxt/",
	"__pycache__/",
	"*.min.js",
	"*.min.css",
	"*.pb.go",
	"*_generated.go",
	"*_gen.go",
	"*.generated.*",
	"coverage/",
	".nyc_output/",
	".env",
	".env.*",
	"*.pem",
	"*.key",
	"*.secret",
	"*.secrets",
}

type ignoreFilter struct {
	gi *gitignore.GitIgnore
}

func (f *ignoreFilter) Match(relPath string) bool {
	clean := filepath.ToSlash(relPath)
	for _, n := range neverIgnore {
		if clean == n || strings.HasPrefix(clean, n+"/") {
			return true
		}
	}
	return f.gi.MatchesPath(clean)
}

// readUserPatterns returns only user-written lines from .lgignore: no defaults, no comments, no blank lines.
func readUserPatterns(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, ".lgignore"))
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func loadIgnore(dir string) lang.Ignorer {
	patterns := make([]string, len(defaultPatterns))
	copy(patterns, defaultPatterns)

	data, err := os.ReadFile(filepath.Join(dir, ".lgignore"))
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			patterns = append(patterns, line)
		}
	}

	return &ignoreFilter{gi: gitignore.CompileIgnoreLines(patterns...)}
}
