package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/faizalv/lemongrass/agent"
	"github.com/faizalv/lemongrass/config"
	"github.com/faizalv/lemongrass/gatekeeper"
	"github.com/faizalv/lemongrass/vault"
)

const (
	agentQueryMaxFailures = 5
	agentQueryWindow      = time.Minute
	agentQueryLockout     = 5 * time.Minute
)

func agentSocketPath() string {
	return filepath.Join(config.Dir(), "agent.sock")
}

func cmdAgent(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrass agent run")
		os.Exit(1)
	}
	switch args[0] {
	case "run":
		cmdAgentRun()
	default:
		fmt.Fprintf(os.Stderr, "unknown agent command: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdAgentRun() {
	if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	sockPath := agentSocketPath()
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

	vaultClient := &gatekeeper.Client{SocketPath: vaultSocketPath()}
	svc := agent.NewService(vaultClient)
	limiter := vault.NewFailureLimiter(agentQueryMaxFailures, agentQueryWindow, agentQueryLockout)

	fmt.Printf("lgrass agent: listening on %s\n", sockPath)
	if err := agent.Serve(svc, l, limiter); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
