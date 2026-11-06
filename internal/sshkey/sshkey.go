// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Package sshkey writes a new OpenSSH key pair for the human's SSH pane. It exists so a passphrase
// never travels on a command line, since `ssh-keygen -N` puts it in argv, which other local users
// can read from /proc.
package sshkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Generate writes an ed25519 private key at path (mode 0600) and its public key at path.pub. It
// never replaces a file: an existing private or public file is an error, and a pair left half
// written is removed.
func Generate(path, comment string, passphrase []byte) error {
	if !filepath.IsAbs(path) {
		return errors.New("sshkey: key path must be absolute")
	}
	if strings.ContainsAny(comment, "\r\n") {
		return errors.New("sshkey: comment must be one line")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("sshkey: generate: %w", err)
	}
	var block *pem.Block
	if len(passphrase) > 0 {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, comment, passphrase)
	} else {
		block, err = ssh.MarshalPrivateKey(priv, comment)
	}
	if err != nil {
		return fmt.Errorf("sshkey: encode private key: %w", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return fmt.Errorf("sshkey: encode public key: %w", err)
	}
	line := strings.TrimSuffix(string(ssh.MarshalAuthorizedKey(sshPub)), "\n")
	if comment != "" {
		line += " " + comment
	}

	if err := writeNew(path+".pub", []byte(line+"\n"), 0o644); err != nil {
		return err
	}
	if err := writeNew(path, pem.EncodeToMemory(block), 0o600); err != nil {
		os.Remove(path + ".pub")
		return err
	}
	return nil
}

func writeNew(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("sshkey: %s already exists", path)
		}
		return fmt.Errorf("sshkey: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(path)
		return fmt.Errorf("sshkey: write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return fmt.Errorf("sshkey: write %s: %w", path, err)
	}
	return nil
}
