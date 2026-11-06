// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package sshkey

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestGenerateWithPassphrase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := Generate(path, "me@laptop", []byte("secret")); err != nil {
		t.Fatal(err)
	}
	privBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ssh.ParsePrivateKey(privBytes); err == nil {
		t.Fatal("key parsed without its passphrase")
	}
	signer, err := ssh.ParsePrivateKeyWithPassphrase(privBytes, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	pubLine, err := os.ReadFile(path + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	pub, comment, _, _, err := ssh.ParseAuthorizedKey(pubLine)
	if err != nil {
		t.Fatal(err)
	}
	if comment != "me@laptop" {
		t.Fatalf("comment = %q", comment)
	}
	if !bytes.Equal(pub.Marshal(), signer.PublicKey().Marshal()) {
		t.Fatal("public file does not match the private key")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %v", info.Mode().Perm())
	}
}

func TestGenerateWithoutPassphrase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := Generate(path, "", nil); err != nil {
		t.Fatal(err)
	}
	privBytes, _ := os.ReadFile(path)
	if _, err := ssh.ParsePrivateKey(privBytes); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Generate(path, "", nil)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "keep" {
		t.Fatal("existing key was replaced")
	}
	if _, err := os.Stat(path + ".pub"); !os.IsNotExist(err) {
		t.Fatal("half pair left behind")
	}
}

func TestGenerateRejectsMultilineComment(t *testing.T) {
	if err := Generate(filepath.Join(t.TempDir(), "k"), "a\nb", nil); err == nil {
		t.Fatal("multiline comment accepted")
	}
}
