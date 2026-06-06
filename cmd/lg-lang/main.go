package main

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type parseRequest struct {
	ProjectPath    string   `json:"project_path"`
	IgnorePatterns []string `json:"ignore_patterns"`
}

type parsedNode struct {
	FilePath    string   `json:"file_path"`
	Package     string   `json:"package"`
	Symbol      string   `json:"symbol"`
	Kind        string   `json:"kind"`
	LineStart   int      `json:"line_start"`
	LineEnd     int      `json:"line_end"`
	Receiver    string   `json:"receiver"`
	Signature   string   `json:"signature"`
	Exported    bool     `json:"exported"`
	DependsOn   []string `json:"depends_on"`
	ContentHash string   `json:"content_hash"`
}

type langGroup struct {
	Language string       `json:"language"`
	Nodes    []parsedNode `json:"nodes"`
}

type parseResponse struct {
	Groups []langGroup `json:"groups"`
}

func main() {
	langs := strings.Split(os.Getenv("LG_LANGUAGES"), ",")

	g := newGrammarLoader()
	for _, l := range langs {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if err := g.load(l); err != nil {
			log.Printf("[lg-lang] grammar %s: %v", l, err)
		} else {
			log.Printf("[lg-lang] loaded grammar: %s", l)
		}
		// Vue grammar is loaded automatically alongside TypeScript
		if l == "ts" {
			if err := g.load("vue"); err != nil {
				log.Printf("[lg-lang] vue grammar not available (Vue files will be skipped): %v", err)
			} else {
				log.Printf("[lg-lang] loaded grammar: vue")
			}
		}
	}

	http.HandleFunc("/parse", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req parseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		resp := handleParse(g, req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("[lg-lang] listening on :3000, grammars: %v", g.loaded())
	log.Fatal(http.ListenAndServe(":3000", nil))
}

func handleParse(g *grammarLoader, req parseRequest) parseResponse {
	ig := buildIgnorer(req.IgnorePatterns)
	groups := map[string][]parsedNode{}

	_ = filepath.WalkDir(req.ProjectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(req.ProjectPath, path)
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			if rel != "." && ig.match(rel+"/") {
				return filepath.SkipDir
			}
			return nil
		}
		if ig.match(rel) {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		lang := extToLang(ext, d.Name())
		if lang == "" {
			return nil
		}

		gr := g.get(lang)
		if gr == nil {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var nodes []parsedNode
		switch lang {
		case "php":
			nodes = extractPHP(gr, src, rel, req.ProjectPath)
		case "ts":
			nodes = extractTS(gr, src, rel, req.ProjectPath, ext)
		case "vue":
			nodes = extractVue(gr, g.get("ts"), src, rel)
		case "py":
			nodes = extractPython(gr, src, rel, req.ProjectPath)
		}

		for _, n := range nodes {
			language := nodeLang(lang, ext, n.Kind)
			groups[language] = append(groups[language], n)
		}
		return nil
	})

	var result []langGroup
	for lang, nodes := range groups {
		result = append(result, langGroup{Language: lang, Nodes: nodes})
	}
	return parseResponse{Groups: result}
}

func extToLang(ext, name string) string {
	switch ext {
	case ".php":
		return "php"
	case ".blade": // .blade.php
		return "php"
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return "ts"
	case ".vue":
		return "vue"
	case ".py":
		return "py"
	}
	// .blade.php has double extension
	if strings.HasSuffix(strings.ToLower(name), ".blade.php") {
		return "php"
	}
	return ""
}

func nodeLang(grammarLang, ext, kind string) string {
	switch grammarLang {
	case "php":
		return "php"
	case "ts":
		return "typescript"
	case "vue":
		return "vue"
	case "py":
		return "python"
	}
	return grammarLang
}

// simpleIgnorer applies gitignore-style patterns as prefix/suffix checks.
type simpleIgnorer struct {
	patterns []string
}

var hardcodedIgnore = []string{
	"node_modules/", "vendor/", "dist/", "build/", ".git/",
	"__pycache__/", ".venv/", "venv/", "storage/framework/", "bootstrap/cache/",
	".nuxt/", ".next/", "coverage/", ".cache/",
}

func buildIgnorer(patterns []string) *simpleIgnorer {
	all := make([]string, len(hardcodedIgnore)+len(patterns))
	copy(all, hardcodedIgnore)
	copy(all[len(hardcodedIgnore):], patterns)
	return &simpleIgnorer{patterns: all}
}

func (ig *simpleIgnorer) match(rel string) bool {
	for _, p := range ig.patterns {
		if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(rel, p) || strings.Contains(rel, "/"+p) {
				return true
			}
		} else if strings.HasSuffix(rel, p) || rel == p {
			return true
		}
	}
	return false
}
