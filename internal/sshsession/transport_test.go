// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package sshsession

import (
	"strings"
	"testing"
)

func TestSessionArgs(t *testing.T) {
	got := strings.Join(sessionArgs("web-1", "/h/ssh/cm-%C", false), " ")
	want := "-o ControlMaster=no -o ControlPath=/h/ssh/cm-%C -o BatchMode=yes -s -- web-1 sftp"
	if got != want {
		t.Fatalf("plain argv\n got %s\nwant %s", got, want)
	}
	args := sessionArgs("web-1", "/h/ssh/cm-%C", true)
	head := strings.Join(args[:len(args)-1], " ")
	if head != "-T -o ControlMaster=no -o ControlPath=/h/ssh/cm-%C -o BatchMode=yes -- web-1" {
		t.Fatalf("sudo argv head = %s", head)
	}
	cmd := args[len(args)-1]
	if !strings.HasPrefix(cmd, "sh -c '") || !strings.HasSuffix(cmd, "'") || strings.Count(cmd, "'") != 2 {
		t.Fatalf("sudo command is not one single-quoted sh -c script: %s", cmd)
	}
	for _, p := range sftpServerPaths {
		if !strings.Contains(cmd, p) {
			t.Fatalf("sudo command misses %s", p)
		}
	}
	if !strings.Contains(cmd, `exec sudo -n "$p"`) {
		t.Fatalf("sudo command does not exec sudo -n: %s", cmd)
	}
}

func TestClassify(t *testing.T) {
	for _, c := range []struct {
		stderr string
		sudo   bool
		code   string
	}{
		{"sudo: a password is required", true, CodeSudoPassword},
		{"sudo: sorry, a password is required to run sudo", true, CodeSudoPassword},
		{"sudo-rs: interactive authentication is required", true, CodeSudoPassword},
		{"Warning: Permanently added 'x' to the list of known hosts.\nsudo: a password is required", true, CodeSudoPassword},
		{"alice is not in the sudoers file.\nsudo: user alice is not in the sudoers file", true, CodeSudoRefused},
		{"sudo: sorry, you must have a tty to run sudo", true, CodeSudoRefused},
		{"sh: 1: sudo: not found", true, CodeSudoRefused},
		{"bash: sudo: command not found", true, CodeSudoRefused},
		{noSftpServerMarker + "/usr/lib/openssh/sftp-server", true, CodeNoSftpServer},
		{"mux_client_request_session: session request failed", true, ""},
		{"sudo: a password is required", false, ""},
		{"", true, ""},
	} {
		code, line := classify(c.stderr, c.sudo)
		if code != c.code {
			t.Errorf("%q (sudo %v): code %q, want %q", c.stderr, c.sudo, code, c.code)
		}
		if code != "" && (line == "" || strings.Contains(line, "\n")) {
			t.Errorf("%q: line %q should be the one matched line", c.stderr, line)
		}
	}
}
