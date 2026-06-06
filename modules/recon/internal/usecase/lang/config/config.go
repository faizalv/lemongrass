package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/faizalv/lemongrass/modules/recon/entity"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang"
)

const maxSize = 64 * 1024

type configParser struct{}

func New() lang.Parser               { return &configParser{} }
func (p *configParser) Name() string  { return "config" }
func (p *configParser) Priority() int { return 10 }

func (p *configParser) Detect(dir string) bool {
	found := false
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || found || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if fileKind(rel, d.Name()) != "" {
			found = true
		}
		return nil
	})
	return found
}

func (p *configParser) ParseFiles(dir string, ig lang.Ignorer, paths []string) (*entity.ParseResult, error) {
	pkgMap := make(map[string]*entity.PackageNode)

	for _, relPath := range paths {
		relPath = filepath.ToSlash(relPath)
		if ig.Match(relPath) {
			continue
		}
		name := filepath.Base(relPath)
		k := fileKind(relPath, name)
		if k == "" {
			continue
		}
		absPath := filepath.Join(dir, relPath)
		info, err := os.Stat(absPath)
		if err != nil || info.Size() > maxSize {
			continue
		}
		lineCount, hash := scanFile(absPath)
		dirRel := filepath.ToSlash(filepath.Dir(relPath))
		if dirRel == "." {
			dirRel = ""
		}
		if pkgMap[dirRel] == nil {
			pkgMap[dirRel] = &entity.PackageNode{ImportPath: dirRel, Dir: dirRel}
		}
		pkg := pkgMap[dirRel]
		pkg.Files = append(pkg.Files, entity.FileNode{
			Path: relPath,
			Exports: []entity.Symbol{{
				Name:        name,
				Kind:        k,
				LineStart:   1,
				LineEnd:     lineCount,
				ContentHash: hash,
			}},
		})
	}

	var packages []entity.PackageNode
	for _, pkg := range pkgMap {
		packages = append(packages, *pkg)
	}
	return (&entity.ProjectTree{Language: "config", Packages: packages}).ToParseResult(), nil
}

func (p *configParser) Parse(dir string, ig lang.Ignorer) (*entity.ParseResult, error) {
	pkgMap := make(map[string]*entity.PackageNode)

	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && ig.Match(rel+"/") {
				return filepath.SkipDir
			}
			return nil
		}
		if ig.Match(rel) {
			return nil
		}
		k := fileKind(rel, d.Name())
		if k == "" {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil || info.Size() > maxSize {
			return nil
		}
		lineCount, hash := scanFile(path)
		dirRel := filepath.ToSlash(filepath.Dir(rel))
		if dirRel == "." {
			dirRel = ""
		}
		if pkgMap[dirRel] == nil {
			pkgMap[dirRel] = &entity.PackageNode{ImportPath: dirRel, Dir: dirRel}
		}
		pkg := pkgMap[dirRel]
		pkg.Files = append(pkg.Files, entity.FileNode{
			Path: rel,
			Exports: []entity.Symbol{{
				Name:        d.Name(),
				Kind:        k,
				LineStart:   1,
				LineEnd:     lineCount,
				ContentHash: hash,
			}},
		})
		return nil
	})

	var packages []entity.PackageNode
	for _, pkg := range pkgMap {
		packages = append(packages, *pkg)
	}

	return (&entity.ProjectTree{Language: "config", Packages: packages}).ToParseResult(), nil
}

// fileKind returns the config kind for the given file, or "" if not recognised.
// Specific kinds are matched before the generic config-yaml fallback.
func fileKind(relPath, name string) string {
	if name == "Dockerfile" || strings.HasPrefix(name, "Dockerfile.") {
		return "dockerfile"
	}
	if name == "Makefile" || name == "GNUmakefile" || name == "makefile" {
		return "makefile"
	}
	if name == ".gitlab-ci.yml" {
		return "ci-gitlab"
	}
	if isYAML(name) && strings.HasPrefix(relPath, ".github/workflows/") {
		return "ci-github"
	}
	if isCompose(name) {
		return "compose"
	}
	if isYAML(name) && isAllowedYAMLDir(relPath) {
		return "config-yaml"
	}
	if strings.HasSuffix(name, ".css") {
		return "css"
	}
	return ""
}

func isYAML(name string) bool {
	return strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")
}

func isCompose(name string) bool {
	if !isYAML(name) {
		return false
	}
	return strings.HasPrefix(name, "docker-compose") || strings.HasPrefix(name, "compose")
}

var yamlAllowedPrefixes = []string{
	".github/", "config/", "deploy/", "k8s/", "helm/", "infra/", ".circleci/",
}

func isAllowedYAMLDir(relPath string) bool {
	dir := filepath.ToSlash(filepath.Dir(relPath))
	if dir == "." || dir == "" {
		return true
	}
	for _, prefix := range yamlAllowedPrefixes {
		if strings.HasPrefix(relPath, prefix) {
			return true
		}
	}
	return false
}

func scanFile(path string) (lines int, hash string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 1, ""
	}
	sum := sha256.Sum256(data)
	lineCount := bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lineCount++
	}
	if lineCount == 0 {
		lineCount = 1
	}
	return lineCount, hex.EncodeToString(sum[:])
}
