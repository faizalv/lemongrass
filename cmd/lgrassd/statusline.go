// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/faizalv/lemongrass/internal/agentconf"
	"github.com/faizalv/lemongrass/internal/agentusage"
	"github.com/faizalv/lemongrass/internal/claudeaccount"
)

// cmdStatusline records the vendor's payload and chains the human's command with unchanged stdin and stdout.
// Claude preserves the command's exit status; Antigravity always succeeds to keep its feed enabled.
func cmdStatusline(args []string) {
	if len(args) == 2 && args[0] == "--vendor" && args[1] == "agy" {
		home, err := os.UserHomeDir()
		if err != nil {
			fail(err)
		}
		payload, err := io.ReadAll(os.Stdin)
		if err != nil {
			fail(err)
		}
		os.Exit(runAgyStatusLine(home, os.Getenv(tabIDEnv), payload, time.Now(), os.Stdout, os.Stderr))
	}
	if len(args) != 2 || args[0] != "--account" {
		fmt.Fprintln(os.Stderr, "usage: lgrassd statusline --account <name> | --vendor agy")
		os.Exit(1)
	}
	account := args[1]
	if account != "primary" {
		if err := claudeaccount.ValidName(account); err != nil {
			fail(err)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fail(err)
	}
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail(err)
	}

	recorded := make(chan struct{})
	go func() {
		defer close(recorded)
		now := time.Now()
		if err := agentusage.RecordClaude(home, account, payload, now); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		// Claude Code runs this command as its own child, so the tab id is the one lemongrass gave that Claude.
		if err := agentusage.RecordContext(home, os.Getenv(tabIDEnv), payload, now); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	code := runHumanStatusLine(home, account, payload, os.Stdout, os.Stderr)
	<-recorded
	os.Exit(code)
}

func runHumanStatusLine(home, account string, payload []byte, stdout, stderr io.Writer) int {
	store := agentconf.StatusLineStore{Path: agentconf.StatusLineStorePath(home)}
	command, err := store.Command(account)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if command == "" {
		return 0
	}
	return runStatusLineCommand(command, payload, stdout, stderr)
}

func runStatusLineCommand(command string, payload []byte, stdout, stderr io.Writer) int {
	cmd, err := statusLineCommand(command)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// Claude Code cancels a run that is still going when a new trigger arrives, so the child must not outlive this
	// process.
	err = runTiedToParent(cmd)
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runAgyStatusLine(home, tabID string, payload []byte, now time.Time, stdout, stderr io.Writer) int {
	if agentconf.StatusLineOn(home) {
		if err := agentusage.RecordAgy(home, payload, now); err != nil {
			fmt.Fprintln(stderr, err)
		}
		if err := agentusage.RecordContext(home, tabID, payload, now); err != nil {
			fmt.Fprintln(stderr, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "lgrassd: reading Antigravity statusline settings:", err)
		return 0
	}
	var settings struct {
		StatusLine struct {
			Command string `json:"command"`
		} `json:"statusLine"`
	}
	if json.Unmarshal(data, &settings) != nil {
		fmt.Fprintln(stderr, "lgrassd: invalid Antigravity statusline settings")
		return 0
	}
	if settings.StatusLine.Command != "" {
		// Antigravity disables the statusline after repeated failures, which would stop quota and context updates too.
		runStatusLineCommand(settings.StatusLine.Command, payload, stdout, stderr)
	}
	return 0
}
