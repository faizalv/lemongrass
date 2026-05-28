package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

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
		// compose file missing (e.g. fresh install after .lemongrass was deleted);
		// stop known containers by name so they don't conflict on the next up
		known := []string{"lg-server", "lg-runner", "lg-embed", "lg-postgres"}
		cmd := exec.Command("docker", append([]string{"rm", "-f"}, known...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run() // best-effort; containers may not exist
	}
	stopFsDaemon()
	return nil
}

func stopFsDaemon() {
	pidPath := filepath.Join(config.Dir(), "fs-daemon.pid")
	sockPath := filepath.Join(config.Dir(), "fs.sock")

	data, err := os.ReadFile(pidPath)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	proc.Signal(syscall.SIGTERM)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); os.IsNotExist(err) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	os.Remove(sockPath)
	os.Remove(pidPath)
}
