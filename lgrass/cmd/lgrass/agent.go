package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/faizalv/lemongrass/agent"
	"github.com/faizalv/lemongrass/config"
	"github.com/faizalv/lemongrass/gatekeeper"
	"github.com/faizalv/lemongrass/session"
	"github.com/faizalv/lemongrass/threadsvc"
	"github.com/faizalv/lemongrass/vault"
)

const (
	agentQueryMaxFailures = 5
	agentQueryWindow      = time.Minute
	agentQueryLockout     = 5 * time.Minute

	threadRetryInterval = 30 * time.Second
	threadPruneInterval = time.Hour
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

	startThreadService()

	fmt.Printf("lgrass agent: listening on %s\n", sockPath)
	if err := agent.Serve(svc, l, limiter); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// A failure here is logged and never stops the agent, since the db and rester proxy do not depend on threads.
func startThreadService() {
	store, err := session.Open(session.DBPath(), "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lgrass agent: thread service not started: %v\n", err)
		return
	}
	threadSock := threadsvc.SocketPath()
	os.Remove(threadSock)
	l, err := net.Listen("unix", threadSock)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lgrass agent: thread service not started: %v\n", err)
		store.Close()
		return
	}
	if err := os.Chmod(threadSock, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "lgrass agent: thread service not started: %v\n", err)
		l.Close()
		store.Close()
		return
	}
	svc := threadsvc.NewService(store)
	go svc.Serve(l)
	go svc.Run(context.Background(), threadRetryInterval, threadPruneInterval)
	fmt.Printf("lgrass agent: thread service listening on %s\n", threadSock)
}
