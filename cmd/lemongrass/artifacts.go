package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/infra/lgart"
)

func cmdArtifacts(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: lemongrass artifacts <export|import|inspect> [args...]\n")
		os.Exit(1)
	}
	switch args[0] {
	case "export":
		cmdArtifactsExport(args[1:])
	case "import":
		cmdArtifactsImport(args[1:])
	case "inspect":
		cmdArtifactsInspect(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown artifacts subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdArtifactsExport(args []string) {
	projectID, serverURL := resolveProject()

	outPath := ""
	if len(args) > 0 {
		outPath = args[0]
	} else {
		cwd, _ := os.Getwd()
		outPath = filepath.Base(cwd) + ".lgart"
	}

	resp, err := http.Get(serverURL + fmt.Sprintf("/api/recon/projects/%d/export", projectID))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: server unreachable: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "error: server returned %d: %s\n", resp.StatusCode, body)
		os.Exit(1)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading response: %v\n", err)
		os.Exit(1)
	}

	f, _ := lgart.Decode(data)
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	nodeCount, knowledgeCount := 0, 0
	if f != nil {
		nodeCount = len(f.Nodes)
		knowledgeCount = len(f.Knowledge)
	}
	fmt.Printf("exported %d nodes, %d knowledge entries -> %s\n", nodeCount, knowledgeCount, outPath)
}

func cmdArtifactsImport(args []string) {
	force := false
	dryRun := false
	var positional []string
	for _, a := range args {
		switch a {
		case "--force":
			force = true
		case "--dry-run":
			dryRun = true
		default:
			positional = append(positional, a)
		}
	}

	if len(positional) < 1 {
		fmt.Fprintf(os.Stderr, "usage: lemongrass artifacts import [--force] [--dry-run] <path>\n")
		os.Exit(1)
	}

	data, err := os.ReadFile(positional[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	f, err := lgart.Decode(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid lgart file: %v\n", err)
		os.Exit(1)
	}

	if dryRun {
		fmt.Printf("dry run: would import up to %d nodes, %d knowledge entries\n", len(f.Nodes), len(f.Knowledge))
		return
	}

	projectID, serverURL := resolveProject()

	url := serverURL + fmt.Sprintf("/api/recon/projects/%d/import", projectID)
	if force {
		url += "?force=true"
	}

	resp, err := http.Post(url, "application/octet-stream", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: server unreachable: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "error: server returned %d: %s\n", resp.StatusCode, body)
		os.Exit(1)
	}

	var result lgart.ImportResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("imported %d nodes, %d knowledge entries (%d nodes skipped, %d knowledge skipped)\n",
		result.NodesImported, result.KnowledgeImported, result.NodesSkipped, result.KnowledgeSkipped)
}

func cmdArtifactsInspect(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: lemongrass artifacts inspect <path>\n")
		os.Exit(1)
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	f, err := lgart.Decode(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid lgart file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Origin:   %s\n", strOrDash(f.GeneratedBy))
	fmt.Printf("Exported: %s\n", f.ExportedAt.UTC().Format(time.RFC3339))
	fmt.Println()

	fileCounts := map[string]int{}
	for _, n := range f.Nodes {
		fileCounts[n.File]++
	}
	dirCounts := map[string]int{}
	for path, count := range fileCounts {
		parts := strings.SplitN(path, "/", 3)
		dir := parts[0]
		if len(parts) >= 2 {
			dir = parts[0] + "/" + parts[1] + "/"
		}
		dirCounts[dir] += count
	}
	topDirs := topN(dirCounts, 4)

	fmt.Printf("Nodes:    %d across %d files\n", len(f.Nodes), len(fileCounts))
	if len(topDirs) > 0 {
		parts := make([]string, len(topDirs))
		for i, d := range topDirs {
			parts[i] = fmt.Sprintf("%s (%d)", d.key, d.val)
		}
		fmt.Printf("  Top dirs: %s\n", strings.Join(parts, "  "))
	}

	labelCounts := map[string]int{}
	for _, k := range f.Knowledge {
		for _, l := range k.Labels {
			labelCounts[l]++
		}
	}
	topLabels := topN(labelCounts, 5)
	fmt.Printf("Knowledge: %d entries\n", len(f.Knowledge))
	if len(topLabels) > 0 {
		parts := make([]string, len(topLabels))
		for i, l := range topLabels {
			parts[i] = fmt.Sprintf("%s (%d)", l.key, l.val)
		}
		fmt.Printf("  Labels: %s\n", strings.Join(parts, "  "))
	}
	fmt.Println()

	var warnings []string

	injectionPhrases := []string{
		"ignore previous", "you are now", "your new instructions",
		"disregard", "new rule", "from now on", "system prompt",
	}
	suspiciousLabels := map[string]bool{
		"system": true, "rules": true, "instructions": true,
		"override": true, "prompt": true, "admin": true,
	}

	for _, k := range f.Knowledge {
		lower := strings.ToLower(k.Content)
		if strings.Contains(k.Content, "#lg.") || strings.Contains(k.Content, "#lg!.") {
			preview := truncate(k.Content, 80)
			warnings = append(warnings, fmt.Sprintf("[command-injection] knowledge %q: content contains \"#lg.\"\n    preview: %q", k.Key, preview))
		}
		for _, phrase := range injectionPhrases {
			if strings.Contains(lower, phrase) {
				preview := truncate(k.Content, 80)
				warnings = append(warnings, fmt.Sprintf("[injection-phrase] knowledge %q: contains %q\n    preview: %q", k.Key, phrase, preview))
				break
			}
		}
		if len(k.Content) > 3000 {
			warnings = append(warnings, fmt.Sprintf("[oversized] knowledge %q: %d chars (limit 3000)", k.Key, len(k.Content)))
		}
		for _, label := range k.Labels {
			if suspiciousLabels[strings.ToLower(label)] {
				warnings = append(warnings, fmt.Sprintf("[suspicious-label] knowledge %q: label %q", k.Key, label))
			}
		}
	}

	for _, n := range f.Nodes {
		lower := strings.ToLower(n.Description)
		if strings.Contains(n.Description, "#lg.") || strings.Contains(n.Description, "#lg!.") {
			preview := truncate(n.Description, 80)
			warnings = append(warnings, fmt.Sprintf("[command-injection] node %s:%s:%s: description contains \"#lg.\"\n    preview: %q", n.File, n.Symbol, n.Kind, preview))
		}
		for _, phrase := range injectionPhrases {
			if strings.Contains(lower, phrase) {
				preview := truncate(n.Description, 80)
				warnings = append(warnings, fmt.Sprintf("[injection-phrase] node %s:%s:%s: contains %q\n    preview: %q", n.File, n.Symbol, n.Kind, phrase, preview))
				break
			}
		}
		if len(n.Description) > 500 {
			warnings = append(warnings, fmt.Sprintf("[oversized] node %s:%s:%s: description %d chars (limit 500)", n.File, n.Symbol, n.Kind, len(n.Description)))
		}
	}

	var phantomFiles []string
	for path := range fileCounts {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			phantomFiles = append(phantomFiles, path)
		}
	}
	if len(phantomFiles) > 0 {
		sort.Strings(phantomFiles)
		examples := phantomFiles
		if len(examples) > 5 {
			examples = examples[:5]
		}
		warnings = append(warnings, fmt.Sprintf("[phantom-file] %d node file(s) not found in current directory:\n    %s",
			len(phantomFiles), strings.Join(examples, "\n    ")))
	}

	if len(warnings) == 0 {
		fmt.Println("No suspicious content found.")
		return
	}

	fmt.Printf("WARNINGS (%d found -- review before importing):\n\n", len(warnings))
	for _, w := range warnings {
		fmt.Printf("  %s\n\n", w)
	}
}

type kv struct{ key string; val int }

func topN(m map[string]int, n int) []kv {
	all := make([]kv, 0, len(m))
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].val != all[j].val {
			return all[i].val > all[j].val
		}
		return all[i].key < all[j].key
	})
	if len(all) > n {
		all = all[:n]
	}
	return all
}

func strOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func resolveProject() (int64, string) {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: cannot determine current directory")
		os.Exit(1)
	}
	for {
		cfgPath := filepath.Join(dir, ".lemongrass", "config.json")
		if data, err := os.ReadFile(cfgPath); err == nil {
			var cfg struct {
				ProjectID int64  `json:"project_id"`
				ServerURL string `json:"server_url"`
			}
			if json.Unmarshal(data, &cfg) == nil && cfg.ProjectID > 0 {
				return cfg.ProjectID, cfg.ServerURL
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	fmt.Fprintln(os.Stderr, "error: project not initialised -- run `lemongrass init` first")
	os.Exit(1)
	return 0, ""
}
