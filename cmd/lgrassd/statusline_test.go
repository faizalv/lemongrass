// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/faizalv/lemongrass/internal/agentconf"
	"github.com/faizalv/lemongrass/internal/agentusage"
)

func writeStatusLineStore(t *testing.T, home, body string) {
	t.Helper()
	path := agentconf.StatusLineStorePath(home)
	os.MkdirAll(filepath.Dir(path), 0o700)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAgyStatusLineRecordsAndChainsUserCommand(t *testing.T) {
	payload, err := os.ReadFile("../../internal/agentusage/testdata/agy-statusline.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, settings := range []string{`{"statusLine":{"command":"cat; exit 3"}}`, `{`, `{}`} {
		t.Run(settings, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, ".gemini", "antigravity-cli", "settings.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(settings), 0o600); err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			if code := runAgyStatusLine(home, "agy-tab", payload, time.Now(), &out, &errOut); code != 0 {
				t.Fatalf("exit %d", code)
			}
			if settings == `{"statusLine":{"command":"cat; exit 3"}}` && !bytes.Equal(out.Bytes(), payload) {
				t.Fatal("stdin or stdout changed")
			}
			if _, ok, err := agentusage.ReadAgy(home); err != nil || !ok {
				t.Fatalf("quota: %v %v", ok, err)
			}
			contexts, err := agentusage.ReadContexts(home)
			if err != nil || contexts["agy-tab"].UsedTokens != 7206 {
				t.Fatalf("contexts: %v %v", contexts, err)
			}
		})
	}
}

func TestAgyStatusLineOffStillChainsWithoutRecording(t *testing.T) {
	home := t.TempDir()
	if err := agentconf.WriteAtomic(agentconf.StatusLineOffPath(home), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".gemini", "antigravity-cli", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"statusLine":{"command":"cat"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"quota":{"gemini-weekly":{"remaining_fraction":0.5,"reset_time":"2030-01-01T00:00:00Z"}},"context_window":{"context_window_size":1000,"current_usage":{"input_tokens":50}}}`)
	var out, errOut bytes.Buffer
	if code := runAgyStatusLine(home, "agy-tab", payload, time.Now(), &out, &errOut); code != 0 || !bytes.Equal(out.Bytes(), payload) {
		t.Fatalf("code %d stdout %q", code, out.String())
	}
	if _, ok, err := agentusage.ReadAgy(home); err != nil || ok {
		t.Fatalf("quota recorded while off: %v %v", ok, err)
	}
	contexts, err := agentusage.ReadContexts(home)
	if err != nil || len(contexts) != 0 {
		t.Fatalf("context recorded while off: %v %v", contexts, err)
	}
}

func TestAgyStatusLineWithoutUserSettings(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runAgyStatusLine(t.TempDir(), "agy-tab", []byte(`{}`), time.Now(), &out, &errOut); code != 0 || out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errOut.String())
	}
}

func TestHumanStatusLineGetsStdinAndPassesOutput(t *testing.T) {
	home := t.TempDir()
	writeStatusLineStore(t, home, `{"commands":{"primary":"cat; echo; echo second"}}`)
	var out, errOut bytes.Buffer
	code := runHumanStatusLine(home, "primary", []byte(`{"a":1}`), &out, &errOut)
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, errOut.String())
	}
	if out.String() != "{\"a\":1}\nsecond\n" {
		t.Errorf("stdout %q", out.String())
	}
}

func TestHumanStatusLineExitCodePassesThrough(t *testing.T) {
	home := t.TempDir()
	writeStatusLineStore(t, home, `{"commands":{"work":"echo partial; exit 3"}}`)
	var out, errOut bytes.Buffer
	if code := runHumanStatusLine(home, "work", nil, &out, &errOut); code != 3 {
		t.Errorf("code %d, want 3", code)
	}
}

func TestNoHumanStatusLinePrintsNothing(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runHumanStatusLine(t.TempDir(), "primary", []byte(`{}`), &out, &errOut); code != 0 || out.Len() != 0 {
		t.Errorf("code %d, stdout %q", code, out.String())
	}
}
