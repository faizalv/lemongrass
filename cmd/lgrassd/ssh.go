// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/faizalv/lemongrass/internal/sshkey"
)

var errSSHUsage = errors.New("usage: lgrassd ssh keygen <absolute-path> [--comment <text>], passphrase on stdin")

// The passphrase can be long but never this long; the cap keeps a stray pipe from filling memory.
const maxPassphraseBytes = 4096

func cmdSSH(args []string) {
	if err := runSSH(args, os.Stdin); err != nil {
		if errors.Is(err, errSSHUsage) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fail(err)
	}
}

func runSSH(args []string, stdin io.Reader) error {
	if len(args) < 2 || args[0] != "keygen" {
		return errSSHUsage
	}
	path := args[1]
	comment := ""
	switch rest := args[2:]; {
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
