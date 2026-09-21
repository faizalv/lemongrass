package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeHome(t *testing.T) (*keeper, string) {
	t.Helper()
	home := t.TempDir()
	lgrass := filepath.Join(home, ".local", "bin", "lgrass")
	if err := os.MkdirAll(filepath.Dir(lgrass), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lgrass, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	k := newKeeper(home)
	k.candidates = []string{lgrass}
	return k, lgrass
}

func TestInvalidSettingsAreNeverOverwritten(t *testing.T) {
	k, _ := fakeHome(t)
	if err := os.MkdirAll(filepath.Dir(k.settingsPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	broken := `{"hooks": `
	if err := os.WriteFile(k.settingsPath(), []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := k.reconcile(); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
	got, _ := os.ReadFile(k.settingsPath())
	if string(got) != broken {
		t.Errorf("file rewritten: %q", got)
	}
}

func TestWritePreservesFileMode(t *testing.T) {
	k, _ := fakeHome(t)
	if err := os.MkdirAll(filepath.Dir(k.settingsPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(k.settingsPath(), []byte(`{"editorMode":"normal"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := k.reconcile(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(k.settingsPath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestNoLgrassBinaryRegistersNoHooks(t *testing.T) {
	k, lgrass := fakeHome(t)
	os.Remove(lgrass)
	if err := k.reconcile(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(k.settingsPath()); err == nil {
		t.Error("settings written although no lgrass binary exists")
	}
	if _, err := os.Stat(k.skillPath()); err != nil {
		t.Errorf("skill should still be installed: %v", err)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
}

func TestWatcherHealsDriftWithoutAnyOtherProcess(t *testing.T) {
	k, lgrass := fakeHome(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { k.run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	healthy := func() bool {
		data, err := os.ReadFile(k.settingsPath())
		return err == nil && strings.Contains(string(data), lgrass+" hook PreToolUse") && !strings.Contains(string(data), "Write|Edit")
	}
	waitFor(t, "initial registration", healthy)

	drifted := `{"hooks":{"PreToolUse":[{"matcher":"Write|Edit","hooks":[{"type":"command","command":"` + lgrass + ` hook PreToolUse"}]}]}}`
	if err := writeAtomic(k.settingsPath(), []byte(drifted), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "matcher healed after drift", healthy)

	if err := os.RemoveAll(filepath.Dir(k.skillPath())); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "skill restored after deletion", func() bool {
		data, err := os.ReadFile(k.skillPath())
		return err == nil && string(data) == string(skillContent)
	})
}

func TestWatcherRegistersHooksWhenLgrassAppears(t *testing.T) {
	k, lgrass := fakeHome(t)
	os.Remove(lgrass)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { k.run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	waitFor(t, "skill installed", func() bool { _, err := os.Stat(k.skillPath()); return err == nil })
	if _, err := os.Stat(k.settingsPath()); err == nil {
		t.Fatal("settings written before the binary existed")
	}
	if err := os.WriteFile(lgrass, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "hooks registered after binary appeared", func() bool {
		data, err := os.ReadFile(k.settingsPath())
		return err == nil && strings.Contains(string(data), lgrass+" hook SessionStart")
	})
}
