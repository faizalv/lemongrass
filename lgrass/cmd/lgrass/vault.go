package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/faizalv/lemongrass/config"
	"github.com/faizalv/lemongrass/vault"
)

const (
	vaultAdminMaxFailures = 5
	vaultAdminWindow      = time.Minute
	vaultAdminLockout     = 5 * time.Minute
)

func vaultSocketPath() string {
	return filepath.Join(config.Dir(), "vault.sock")
}

func cmdVault(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrass vault run")
		os.Exit(1)
	}
	switch args[0] {
	case "run":
		cmdVaultRun()
	default:
		fmt.Fprintf(os.Stderr, "unknown vault command: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdVaultRun() {
	hardenProcess()

	if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	sockPath := vaultSocketPath()
	os.Remove(sockPath) // a stale socket left behind by a previous, uncleanly-stopped run

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer l.Close()
	if err := os.Chmod(sockPath, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	svc, err := vault.NewService(filepath.Join(config.Dir(), "vault"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	limiter := vault.NewFailureLimiter(vaultAdminMaxFailures, vaultAdminWindow, vaultAdminLockout)

	fmt.Printf("lgrass vault: listening on %s\n", sockPath)
	if err := vault.Serve(svc, l, limiter); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
