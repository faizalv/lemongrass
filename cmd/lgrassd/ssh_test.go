// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestSSHSessionArgs(t *testing.T) {
	for _, args := range [][]string{
		{"session"},
		{"session", "web-1"},
		{"session", "--control-path", "/x/cm", "web-1"},
		{"session", "web-1", "--control-path"},
		{"session", "web-1", "--control-path", "/x/cm", "extra"},
		{"session", "web-1", "--sudo"},
		{"session", "web-1", "--control-path", "/x/cm", "--sudo", "--sudo"},
		{"session", "web-1", "--control-path", "/x/cm", "--control-path", "/y/cm"},
		{"bogus"},
		{},
	} {
		if err := runSSH(args, strings.NewReader(""), &bytes.Buffer{}); !errors.Is(err, errSSHUsage) {
			t.Errorf("%q: want usage error, got %v", args, err)
		}
	}
	if err := runSSH([]string{"session", "-oProxyCommand=x", "--control-path", "/x/cm"}, strings.NewReader(""), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "invalid host alias") {
		t.Errorf("option-like alias accepted: %v", err)
	}
	if err := runSSH([]string{"session", "web-1", "--control-path", "cm"}, strings.NewReader(""), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Errorf("relative control path accepted: %v", err)
	}
	// With stdin already closed the session exits at once without dialing.
	for _, args := range [][]string{
		{"session", "web-1", "--control-path", "/x/cm"},
		{"session", "web-1", "--control-path", "/x/cm", "--sudo"},
		{"session", "web-1", "--sudo", "--control-path", "/x/cm"},
	} {
		if err := runSSH(args, strings.NewReader(""), &bytes.Buffer{}); err != nil {
			t.Errorf("%q on closed stdin: %v", args, err)
		}
	}
}
