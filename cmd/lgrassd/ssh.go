// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/faizalv/lemongrass/internal/sshkey"
	"github.com/faizalv/lemongrass/internal/sshsession"
)

var errSSHUsage = errors.New("usage: lgrassd ssh keygen <absolute-path> [--comment <text>], passphrase on stdin\n" +
	"       lgrassd ssh session <alias> --control-path <absolute-path> [--sudo], JSON lines on stdin and stdout")

// The passphrase can be long but never this long; the cap keeps a stray pipe from filling memory.
const maxPassphraseBytes = 4096

func cmdSSH(args []string) {
	if err := runSSH(args, os.Stdin, os.Stdout); err != nil {
		if errors.Is(err, errSSHUsage) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fail(err)
	}
}

func runSSH(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return errSSHUsage
	}
	switch args[0] {
	case "keygen":
		return runSSHKeygen(args[1:], stdin)
	case "session":
		return runSSHSession(args[1:], stdin, stdout)
	}
	return errSSHUsage
}

// runSSHSession serves one SSH tab's file browser until main closes stdin. --sudo makes the file
// browser act as root through passwordless sudo.
func runSSHSession(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) < 1 {
		return errSSHUsage
	}
	alias, controlPath, sudo := args[0], "", false
	for rest := args[1:]; len(rest) > 0; {
		switch {
		case rest[0] == "--sudo" && !sudo:
			sudo, rest = true, rest[1:]
		case rest[0] == "--control-path" && controlPath == "" && len(rest) > 1:
			controlPath, rest = rest[1], rest[2:]
		default:
			return errSSHUsage
		}
	}
	if controlPath == "" {
		return errSSHUsage
	}
	if !sshsession.ValidAlias(alias) {
		return fmt.Errorf("lgrassd: invalid host alias %q", alias)
	}
	if !filepath.IsAbs(controlPath) {
		return errors.New("lgrassd: control path must be absolute")
	}
	return sshsession.New(sshsession.SSHDialer(alias, controlPath, sudo)).Serve(stdin, stdout)
}

func runSSHKeygen(args []string, stdin io.Reader) error {
	if len(args) < 1 {
		return errSSHUsage
	}
	path := args[0]
	comment := ""
	switch rest := args[1:]; {
	case len(rest) == 0:
	case len(rest) == 2 && rest[0] == "--comment":
		comment = rest[1]
	default:
		return errSSHUsage
	}
	raw, err := io.ReadAll(io.LimitReader(stdin, maxPassphraseBytes+1))
	if err != nil {
		return fmt.Errorf("lgrassd: read passphrase: %w", err)
	}
	if len(raw) > maxPassphraseBytes {
		return errors.New("lgrassd: passphrase too long")
	}
	passphrase := []byte(strings.TrimRight(string(raw), "\r\n"))
	return sshkey.Generate(path, comment, passphrase)
}
