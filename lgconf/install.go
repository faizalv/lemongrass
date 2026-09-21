package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const unitName = "lgconf.service"

func unitText(binary string) string {
	return fmt.Sprintf(`[Unit]
Description=lemongrass Claude Code config keeper

[Service]
Type=simple
ExecStart=%s run
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
`, binary)
}

func installUnit(home string) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(binary); err == nil {
		binary = resolved
	}

	unitPath := filepath.Join(home, ".config", "systemd", "user", unitName)
	if err := writeAtomic(unitPath, []byte(unitText(binary)), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"daemon-reload"},
		{"enable", unitName},
		{"restart", unitName},
	} {
		cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("systemctl --user %s: %w", args[0], err)
		}
	}
	return nil
}
