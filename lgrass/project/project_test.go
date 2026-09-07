package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegisterCreatesEntryMatchingElectronUIShape(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()

	p, err := Register(dir)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if p.ID == "" {
		t.Error("Register did not assign an id")
	}
	if p.Name != filepath.Base(dir) {
		t.Errorf("Name = %q, want %q (basename)", p.Name, filepath.Base(dir))
	}
	if p.Path != dir {
		t.Errorf("Path = %q, want %q", p.Path, dir)
	}

	resolved, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve after Register: %v", err)
	}
	if resolved.ID != p.ID {
		t.Errorf("Resolve returned a different project than Register created: %+v vs %+v", resolved, p)
	}
}

func TestRegisterIsIdempotentByExactPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()

	first, err := Register(dir)
	if err != nil {
		t.Fatalf("Register (first): %v", err)
	}
	second, err := Register(dir)
	if err != nil {
		t.Fatalf("Register (second): %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("Register on an already-registered path returned a new id: %q vs %q", first.ID, second.ID)
	}

	data, err := os.ReadFile(registryPath())
	if err != nil {
		t.Fatalf("reading registry: %v", err)
	}
	var projects []Project
	if err := json.Unmarshal(data, &projects); err != nil {
		t.Fatalf("parsing registry: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("registry has %d entries after registering the same path twice, want 1", len(projects))
	}
}
