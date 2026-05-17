package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/faizalv/lemongrass/config"
)

func cmdRemount(paths []string) {
	cfg := config.LoadOrDefault()
	composePath := filepath.Join(config.Dir(), "docker-compose.yml")

	if err := os.WriteFile(composePath, config.GenerateCompose(cfg, paths), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write compose: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "--wait", "lg-server")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "remount failed: %v\n", err)
		os.Exit(1)
	}
}
