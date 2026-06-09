package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/config"
)

type lgProjectConfig struct {
	ProjectID int64  `json:"project_id"`
	ServerURL string `json:"server_url"`
}

func cmdInit(args []string) {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}

	abs, err := filepath.Abs(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(abs); err != nil {
		fmt.Fprintf(os.Stderr, "error: path does not exist: %s\n", abs)
		os.Exit(1)
	}

	ok, warnings := validateProjectDir(abs)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	if !ok && len(warnings) == 0 {
		fmt.Fprintln(os.Stderr, "warning: this directory may not be a suitable project root")
	}

	lgConfigPath := filepath.Join(abs, ".lemongrass", "config.json")
	if data, err := os.ReadFile(lgConfigPath); err == nil {
		var existing lgProjectConfig
		if json.Unmarshal(data, &existing) == nil && existing.ProjectID > 0 {
			fmt.Printf("already initialised (project %d)\n", existing.ProjectID)
			return
		}
	}

	cfg := config.LoadOrDefault()
	serverURL := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)

	body, _ := json.Marshal(map[string]string{"path": abs})
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(serverURL+"/api/fs/projects", "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: lg-server unreachable -- run `lemongrass up` first")
		os.Exit(1)
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		fmt.Fprintf(os.Stderr, "error: server returned %d\n", resp.StatusCode)
		os.Exit(1)
	}

	projectID := fetchProjectID(client, serverURL, abs)
	if projectID == 0 {
		fmt.Fprintln(os.Stderr, "error: could not resolve project ID from server")
		os.Exit(1)
	}

	lgDir := filepath.Join(abs, ".lemongrass")
	if err := os.MkdirAll(lgDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	lgCfgData, _ := json.MarshalIndent(lgProjectConfig{ProjectID: projectID, ServerURL: serverURL}, "", "  ")
	if err := os.WriteFile(lgConfigPath, lgCfgData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	addToGitignore(abs, ".lemongrass/")

	fmt.Printf("mounting %s into lg-server...\n", abs)
	paths := queryProjectPaths()
	cmdRemount(paths)

	fmt.Println("You're good to go")
}

func fetchProjectID(client *http.Client, serverURL, path string) int64 {
	resp, err := client.Get(serverURL + "/api/fs/projects")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var projects []struct {
		ID   int64  `json:"id"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return 0
	}
	for _, p := range projects {
		if p.Path == path {
			return p.ID
		}
	}
	return 0
}

func addToGitignore(dir, entry string) {
	gitignorePath := filepath.Join(dir, ".gitignore")
	data, _ := os.ReadFile(gitignorePath)
	if strings.Contains(string(data), entry) {
		return
	}
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		f.WriteString("\n")
	}
	f.WriteString(entry + "\n")
}
