package main

import (
	"testing"

	"github.com/faizalv/lemongrass/internal/session"
)

const testProjectID = "cmdtestproj"

func openTestStore(t *testing.T) *session.Store {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	store, err := session.Open(session.DBPath(), testProjectID)
	if err != nil {
		t.Fatalf("session.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}
