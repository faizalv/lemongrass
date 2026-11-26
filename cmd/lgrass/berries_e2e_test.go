// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/faizalv/lemongrass/internal/berries"
	"github.com/faizalv/lemongrass/internal/project"
	"github.com/faizalv/lemongrass/internal/vault"
	"github.com/faizalv/lemongrass/internal/vault/vaulttest"
)

const e2eMap = `roles: [customer, admin]
routes:
  - route: GET /invoices/{id}
    allow: [owner, admin]
    source: PRD invoices
    status: approved
  - route: GET /admin/stats
    allow: [admin]
    source: PRD admin
    status: approved
  - route: GET /me
    allow: [any]
    source: human
    status: approved
`

// shopAPI is a fixture with two customers and an admin. Its invoice route forgets the ownership check until fixed is
// set, which is the one hole a run must find.
func shopAPI(fixed *atomic.Bool) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		json.NewEncoder(w).Encode(map[string]string{"access_token": body["u"]})
	})
	reply := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}
	caller := func(r *http.Request) string { return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") }
	mux.HandleFunc("/invoices/1", func(w http.ResponseWriter, r *http.Request) {
		if fixed.Load() && caller(r) != "alice" && caller(r) != "root" {
			reply(w, 404, map[string]string{"error": "not found"})
			return
		}
		reply(w, 200, map[string]any{"id": 1, "owner": "alice", "total": 10})
	})
	mux.HandleFunc("/admin/stats", func(w http.ResponseWriter, r *http.Request) {
		if caller(r) != "root" {
			reply(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		reply(w, 200, map[string]int{"users": 3})
	})
	mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]string{"name": caller(r)})
	})
	return httptest.NewServer(mux)
}

func TestBerriesFindsTheMissingOwnershipCheckAndSeesItFixed(t *testing.T) {
	var fixed atomic.Bool
	srv := shopAPI(&fixed)
	defer srv.Close()

	vaultClient, agentClient := startStack(t)
	const passphrase = "correct horse"
	domain := vaulttest.Domain{
		BaseURL:         srv.URL,
		LoginEndpoint:   "/login",
		TokenPath:       "access_token",
		TokenPlacement:  vault.TokenPlacement{Kind: vault.PlacementHeader, Name: "Authorization", Prefix: "Bearer "},
		FixedTTLSeconds: 3600,
		Users: []vault.ResterUser{
			{Name: "alice", Fields: map[string]string{"u": "alice"}, Tags: []string{"customer"}},
			{Name: "bob", Fields: map[string]string{"u": "bob"}, Tags: []string{"customer"}},
			{Name: "root", Fields: map[string]string{"u": "root"}, Tags: []string{"admin"}},
		},
	}
	if err := vaultClient.PutRester(passphrase, "shop", domain.Rester()); err != nil {
		t.Fatal(err)
	}
	c, err := vaultClient.CreateHTTPChannel(passphrase, "ch", "shop", vault.DefaultEnvironment, vault.TypeAPI, vault.HTTPScope{Methods: []string{"GET"}}, nil, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	shortID, err := agentClient.RegisterChannel(c.ID)
	if err != nil {
		t.Fatal(err)
	}

	proj := project.Project{ID: "p1", Path: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(proj.Path, ".berries"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj.Path, berries.MapFile), []byte(e2eMap), 0o644); err != nil {
		t.Fatal(err)
	}
	store := berries.Store{Dir: t.TempDir()}
	restore := seedStore
	seedStore = func() (berries.Store, bool) { return store, true }
	defer func() { seedStore = restore }()

	// The calls a model would make while trying its work, recorded as seeds by lgrass rester itself.
	for _, args := range [][]string{
		{shortID, "get", "/invoices/1", "--user", "Alice"},
		{shortID, "get", srv.URL + "/admin/stats", "--user", "root"},
		{shortID, "get", "me", "--user", "alice"},
	} {
		cmd, err := parseResterArgs(args)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := execRester(agentClient, cmd); err != nil {
			t.Fatal(err)
		}
	}
	seeds, err := store.Seeds()
	if err != nil || len(seeds) != 3 {
		t.Fatalf("seeds = %+v, %v", seeds, err)
	}
	for _, s := range seeds {
		if s.Rester != "shop" || s.Environment != vault.DefaultEnvironment || !strings.HasPrefix(s.Path, "/") {
			t.Errorf("seed %+v", s)
		}
	}

	out, err := runBerries(agentClient, proj, store, []string{"run", shortID})
	if err != nil {
		t.Fatal(err)
	}
	stains, _ := store.Stains()
	if len(stains) != 1 {
		t.Fatalf("run 1 stains = %+v\n%s", stains, out)
	}
	s := stains[0]
	if s.Status != berries.StatusNew || s.Finding.Check != berries.CheckHorizontal || s.Finding.State != berries.StateConfirmed ||
		s.Finding.Route != "GET /invoices/{id}" || s.Finding.Owner != "alice" || s.Finding.User != "bob" {
		t.Fatalf("stain = %+v\n%s", s, out)
	}
	if !strings.Contains(out, s.ID) || !strings.Contains(out, "1 new") {
		t.Errorf("run output does not name the stain:\n%s", out)
	}

	detail, err := runBerries(agentClient, proj, store, []string{"stain", s.ID})
	if err != nil || !strings.Contains(detail, "control, as alice: status 200") || !strings.Contains(detail, "replay, as bob: status 200") {
		t.Errorf("stain detail = %v\n%s", err, detail)
	}

	fixed.Store(true)
	out, err = runBerries(agentClient, proj, store, []string{"replay", shortID, s.ID})
	if err != nil {
		t.Fatal(err)
	}
	stains, _ = store.Stains()
	if stains[0].Status != berries.StatusFixed {
		t.Fatalf("after the fix: %+v\n%s", stains[0], out)
	}

	fixed.Store(false)
	if _, err := runBerries(agentClient, proj, store, []string{"run", shortID}); err != nil {
		t.Fatal(err)
	}
	stains, _ = store.Stains()
	if len(stains) != 1 || stains[0].Status != berries.StatusRegressed {
		t.Fatalf("after the check was removed again: %+v", stains)
	}

	mapOut, err := runBerries(agentClient, proj, store, []string{"map", shortID})
	if err != nil || !strings.Contains(mapOut, "role admin has one user") || !strings.Contains(mapOut, "GET /invoices/{id} allows owner, admin, approved, 1 seeds") {
		t.Errorf("map = %v\n%s", err, mapOut)
	}
}
