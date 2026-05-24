package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/faizalv/lemongrass/config"
)

func cmdStatus() {
	config.EnsureScaffold()

	composePath := filepath.Join(config.Dir(), "docker-compose.yml")
	if _, err := os.Stat(composePath); err != nil {
		fmt.Println("=== Containers ===")
		fmt.Println("  (not started; run: lemongrass up)")
		return
	}

	fmt.Println("=== Containers ===")
	cmd := exec.Command("docker", "compose", "-f", composePath, "ps")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}
