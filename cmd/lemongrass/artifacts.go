package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
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
	label := ""
	var positional []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--label" && i+1 < len(args) {
			label = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--label=") {
			label = strings.TrimPrefix(args[i], "--label=")
		} else {
			positional = append(positional, args[i])
		}
	}

	cfg := resolveProject()

	gitOrigin := gitOutput(cfg.ProjectDir, "remote", "get-url", "origin")
	gitUser := gitOutput(cfg.ProjectDir, "config", "user.name")

	if gitOrigin == "" && label == "" {
		fmt.Fprintln(os.Stderr, "error: no git remote found. Provide a label with --label <name>")
		os.Exit(1)
	}

	outPath := ""
	if len(positional) > 0 {
		outPath = positional[0]
	} else {
		outPath = filepath.Base(cfg.ProjectDir) + ".lgart"
	}

	q := url.Values{}
	if label != "" {
		q.Set("project_label", label)
	}
	if gitOrigin != "" {
		q.Set("git_origin", gitOrigin)
	}
	if gitUser != "" {
		q.Set("git_user", gitUser)
	}

	fmt.Print("Exporting... ")
	resp, err := http.Get(cfg.ServerURL + fmt.Sprintf("/api/recon/projects/%d/export?%s", cfg.ProjectID, q.Encode()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nerror: server unreachable: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "\nerror: server returned %d: %s\n", resp.StatusCode, body)
		os.Exit(1)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nerror: reading response: %v\n", err)
		os.Exit(1)
	}

	f, _ := lgart.Decode(data)
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "\nerror: %v\n", err)
		os.Exit(1)
	}

	nodeCount, knowledgeCount := 0, 0
	if f != nil {
		nodeCount = len(f.Nodes)
		knowledgeCount = len(f.Knowledge)
	}
	fmt.Printf("done\nexported %d nodes, %d knowledge entries -> %s\n", nodeCount, knowledgeCount, outPath)
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

	cfg := resolveProject()

	fmt.Println()
	if f.ProjectLabel != "" {
		fmt.Printf("  Label:    %s\n", f.ProjectLabel)
	}
	if f.GitOrigin != "" {
		fmt.Printf("  Origin:   %s\n", f.GitOrigin)
	}
	if f.GitUser != "" {
		fmt.Printf("  User:     %s\n", f.GitUser)
	}
	fmt.Printf("  Exported: %s\n", f.ExportedAt.UTC().Format(time.RFC3339))
	fmt.Printf("  Contents: %d nodes, %d knowledge entries\n", len(f.Nodes), len(f.Knowledge))
	fmt.Printf("  Target:   %s\n", cfg.ProjectDir)
	fmt.Println()

	keys := sampleNodeKeys(f.Nodes, 50)
	if len(keys) > 0 {
		ov := checkOverlap(cfg, keys)
		if !ov.Ready {
			fmt.Fprintln(os.Stderr, "error: semantic map not ready -- run `lemongrass up` and wait for the initial sync to complete")
			os.Exit(1)
		}
		fmt.Printf("  Overlap:  %d/%d symbols found in local semantic map\n\n", ov.Matched, ov.Total)
		if ov.Total > 0 && float64(ov.Matched)/float64(ov.Total) < 0.15 {
			fmt.Printf("  WARNING: only %d/%d sampled symbols matched. This lgart may not belong to this project.\n\n", ov.Matched, ov.Total)
			if !dryRun {
				fmt.Print("Type 'yes' to import anyway: ")
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()
				if strings.TrimSpace(strings.ToLower(scanner.Text())) != "yes" {
					fmt.Println("import cancelled")
					return
				}
				fmt.Println()
			}
		}
	}

	if dryRun {
		withHash := 0
		for _, n := range f.Nodes {
			if n.ContentHash != "" {
				withHash++
			}
		}
		withoutHash := len(f.Nodes) - withHash
		fmt.Printf("dry run:\n")
		fmt.Printf("  would process  %d nodes (%d with content_hash, %d v1-matched)\n", len(f.Nodes), withHash, withoutHash)
		fmt.Printf("  would process  %d knowledge entries\n", len(f.Knowledge))
		return
	}

	importURL := cfg.ServerURL + fmt.Sprintf("/api/recon/projects/%d/import", cfg.ProjectID)
	if force {
		importURL += "?force=true"
	}
	resp, err := http.Post(importURL, "application/octet-stream", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: server unreachable: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		var e struct{ Error string `json:"error"` }
		json.Unmarshal(body, &e)
		fmt.Fprintf(os.Stderr, "error: %s\n", e.Error)
		os.Exit(1)
	}
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

	if result.NodesInserted > 0 {
		fmt.Printf("imported %d nodes, inserted %d nodes, %d knowledge entries (%d nodes skipped, %d knowledge skipped)\n",
			result.NodesImported, result.NodesInserted, result.KnowledgeImported, result.NodesSkipped, result.KnowledgeSkipped)
	} else {
		fmt.Printf("imported %d nodes, %d knowledge entries (%d nodes skipped, %d knowledge skipped)\n",
			result.NodesImported, result.KnowledgeImported, result.NodesSkipped, result.KnowledgeSkipped)
	}
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

	if f.ProjectLabel != "" {
		fmt.Printf("Label:    %s\n", f.ProjectLabel)
	}
	if f.GitOrigin != "" {
		fmt.Printf("Origin:   %s\n", f.GitOrigin)
	}
	if f.GitUser != "" {
		fmt.Printf("User:     %s\n", f.GitUser)
	}
	fmt.Printf("Version:  %d\n", f.Version)
	fmt.Printf("Exported: %s\n", f.ExportedAt.UTC().Format(time.RFC3339))
	if f.EmbedModel != "" {
		fmt.Printf("Embed:    %s\n", f.EmbedModel)
	}
	fmt.Println()

	fileCounts := map[string]int{}
	branchCounts := map[string]int{}
	withHash, withHashEmbed, withoutHash := 0, 0, 0
	for _, n := range f.Nodes {
		fileCounts[n.File]++
		if n.ContentHash != "" {
			if len(n.Embedding) > 0 {
				withHashEmbed++
			} else {
				withHash++
			}
			for _, b := range n.Branches {
				branchCounts[b]++
			}
		} else {
			withoutHash++
		}
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

	fmt.Printf("Nodes:     %d across %d files\n", len(f.Nodes), len(fileCounts))
	if f.Version >= 2 {
		fmt.Printf("  v2 breakdown: %d hash+embed, %d hash only, %d without hash\n", withHashEmbed, withHash, withoutHash)
	}
	if len(topDirs) > 0 {
		parts := make([]string, len(topDirs))
		for i, d := range topDirs {
			parts[i] = fmt.Sprintf("%s (%d)", d.key, d.val)
		}
		fmt.Printf("  Top dirs: %s\n", strings.Join(parts, "  "))
	}
	if len(branchCounts) > 0 {
		topBranches := topN(branchCounts, 5)
		parts := make([]string, len(topBranches))
		for i, b := range topBranches {
			parts[i] = fmt.Sprintf("%s (%d)", b.key, b.val)
		}
		fmt.Printf("  Branches: %s\n", strings.Join(parts, "  "))
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

	if f.Version >= 2 && withoutHash > 0 {
		warnings = append(warnings, fmt.Sprintf("[missing-hash] %d nodes in a v2 file have no content_hash -- will be matched as v1 entries on import", withoutHash))
	}

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
		for _, phrase := range injectionPhrases {
			if strings.Contains(lower, phrase) {
				warnings = append(warnings, fmt.Sprintf("[injection-phrase] knowledge %q contains %q\n    preview: %q", k.Key, phrase, truncate(k.Content, 80)))
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
		for _, phrase := range injectionPhrases {
			if strings.Contains(lower, phrase) {
				warnings = append(warnings, fmt.Sprintf("[injection-phrase] node %s:%s:%s contains %q\n    preview: %q", n.File, n.Symbol, n.Kind, phrase, truncate(n.Description, 80)))
				break
			}
		}
		if len(n.Description) > 500 {
			warnings = append(warnings, fmt.Sprintf("[oversized] node %s:%s:%s: description %d chars (limit 500)", n.File, n.Symbol, n.Kind, len(n.Description)))
		}
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

type overlapResult struct {
	Matched int  `json:"matched"`
	Total   int  `json:"total"`
	Ready   bool `json:"ready"`
}

func sampleNodeKeys(nodes []lgart.Node, max int) []string {
	keys := make([]string, 0, len(nodes))
	for _, n := range nodes {
		keys = append(keys, n.File+":"+n.Symbol+":"+n.Kind)
	}
	if len(keys) > max {
		keys = keys[:max]
	}
	return keys
}

func checkOverlap(cfg lgProjectConfig, keys []string) overlapResult {
	body, _ := json.Marshal(map[string][]string{"keys": keys})
	resp, err := http.Post(
		cfg.ServerURL+fmt.Sprintf("/api/recon/projects/%d/import/overlap", cfg.ProjectID),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return overlapResult{Ready: true}
	}
	defer resp.Body.Close()
	var result overlapResult
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func gitOutput(dir string, args ...string) string {
	allArgs := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", allArgs...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

type kv struct {
	key string
	val int
}

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

func resolveProject() lgProjectConfig {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: cannot determine current directory")
		os.Exit(1)
	}
	for {
		cfgPath := filepath.Join(dir, ".lemongrass", "config.json")
		if data, err := os.ReadFile(cfgPath); err == nil {
			var cfg lgProjectConfig
			if json.Unmarshal(data, &cfg) == nil && cfg.ProjectID > 0 {
				return cfg
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
	return lgProjectConfig{}
}
