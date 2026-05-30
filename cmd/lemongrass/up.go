package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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

	cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "--wait")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start containers: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Lemongrass is running at http://%s:%d\n", cfg.Host, cfg.Port)
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
