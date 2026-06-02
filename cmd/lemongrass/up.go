package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/faizalv/lemongrass/config"
)

func cmdUp() {
	config.EnsureScaffold()

	credFile := filepath.Join(config.Dir(), "claude", ".credentials.json")
	info, err := os.Stat(credFile)
	if err != nil || info.Size() == 0 {
		fmt.Fprintln(os.Stderr, "error: no credentials found. Run: lemongrass auth")
		os.Exit(1)
	}

	teardown()

	cfg := config.LoadOrDefault()
	if cfg.HomeDir == "" {
		cfg.HomeDir, _ = os.UserHomeDir()
	}
	if cfg.BinPath == "" {
		cfg.BinPath = config.DetectBinPath()
	}
	config.Save(cfg)

	writeHookSettings(cfg)
	installAndStartDaemon(cfg.BinPath)

	composePath := filepath.Join(config.Dir(), "docker-compose.yml")
	if err := os.WriteFile(composePath, config.GenerateCompose(cfg, nil), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write compose file: %v\n", err)
		os.Exit(1)
	}

	pgUp := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "--wait", "lg-postgres")
	pgUp.Stdout = os.Stdout
	pgUp.Stderr = os.Stderr
	if err := pgUp.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres: %v\n", err)
		os.Exit(1)
	}

	projectPaths := queryProjectPaths()

	if err := os.WriteFile(composePath, config.GenerateCompose(cfg, projectPaths), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write compose file: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "--wait")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start containers: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Lemongrass is running at http://%s:%d\n", cfg.Host, cfg.Port)
}

func queryProjectPaths() []string {
	out, err := exec.Command("docker", "exec", "lg-postgres",
		"psql", "-U", "lemongrass", "-tAc",
		"SELECT path FROM lg_projects WHERE status != 'removed'",
	).Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if p := strings.TrimSpace(line); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func writeHookSettings(cfg config.Config) {
	claudeDir := filepath.Join(cfg.HomeDir, ".lemongrass", "claude")
	if cfg.HomeDir == "" {
		claudeDir = filepath.Join(config.Dir(), "claude")
	}
	settings := `{
  "permissions": {
    "allow": ["Write", "Edit"]
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "lg-hook"}]
      },
      {
        "matcher": "Write",
        "hooks": [{"type": "command", "command": "lg-hook"}]
      },
      {
        "matcher": "Read",
        "hooks": [{"type": "command", "command": "lg-hook"}]
      }
    ]
  }
}`
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0644)
}
