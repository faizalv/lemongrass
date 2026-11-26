// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package project

import (
	"path/filepath"
	"testing"

	"github.com/faizalv/lemongrass/internal/testenv"
)

func TestResolveMany(t *testing.T) {
	testenv.Home(t)
	a, err := Register(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Register(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got := ResolveMany([]string{
		filepath.Join(b.Path, "src", "missing.go"),
		a.Path,
		filepath.Join(b.Path, "x"),
		t.TempDir(),
		"relative/path",
	})
	if len(got) != 2 || got[0].ID != b.ID || got[1].ID != a.ID {
		t.Errorf("ResolveMany = %+v, want b then a", got)
	}
	if ResolveMany(nil) != nil {
		t.Errorf("ResolveMany(nil) is not nil")
	}
}
