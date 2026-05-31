package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/faizalv/lemongrass/config"
)

func cmdDown() {
	if err := teardown(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop containers: %v\n", err)
		os.Exit(1)
	}
}

func teardown() error {
	composePath := filepath.Join(config.Dir(), "docker-compose.yml")
	if _, err := os.Stat(composePath); err == nil {
		cmd := exec.Command("docker", "compose", "-f", composePath, "down")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	} else {
		known := []string{"lg-server", "lg-runner", "lg-embed", "lg-postgres"}
		cmd := exec.Command("docker", append([]string{"rm", "-f"}, known...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = io.Discard
		_ = cmd.Run()
	}
	uninstallDaemon()
	return nil
}
