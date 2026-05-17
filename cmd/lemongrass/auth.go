package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/faizalv/lemongrass/config"
)

func cmdAuth() {
	config.EnsureScaffold()

	claudeDir := filepath.Join(config.Dir(), "claude")

	cmd := exec.Command("docker", "run", "--rm", "-it",
		"-v", claudeDir+":/home/lg/.lemongrass/claude",
		"-e", "CLAUDE_CONFIG_DIR=/home/lg/.lemongrass/claude",
		"lemongrass-runner:latest",
		"claude",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "auth session ended: %v\n", err)
	}

	credFile := filepath.Join(claudeDir, ".credentials.json")
	info, err := os.Stat(credFile)
	if err != nil || info.Size() == 0 {
		fmt.Fprintln(os.Stderr, "auth failed: credentials not found in ~/.lemongrass/claude/")
		os.Exit(1)
	}

	fmt.Println("Auth successful. Run lemongrass up to start.")
}
