// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package berries

import (
	"strings"
	"testing"
)

const sampleMap = `
roles: [customer, admin]
routes:
  - route: GET /invoices/{id}
    allow: [owner, admin]
    source: PRD invoices
    status: approved
  - route: GET /invoices/export
    allow: [admin]
    source: PRD invoices
    status: proposed
  - route: get /me
    allow: [any]
    source: human
    status: approved
`

func TestParseMapAndMatchPrefersLiteralSegments(t *testing.T) {
	m, err := ParseMap([]byte(sampleMap))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"/invoices/42":     "GET /invoices/{id}",
		"/invoices/export": "GET /invoices/export",
		"/invoices/42/":    "GET /invoices/{id}",
		"/me":              "get /me",
	}
	for path, want := range cases {
		r, ok := m.Match("GET", path)
		if !ok || r.Pattern != want {
			t.Errorf("Match(GET %s) = %v, %v; want %s", path, r, ok, want)
		}
	}
	if _, ok := m.Match("POST", "/invoices/42"); ok {
		t.Error("a POST matched a GET route")
	}
	if _, ok := m.Match("GET", "/invoices/42/pdf"); ok {
		t.Error("a longer path matched a shorter route")
	}
}

func TestEmptyMapParses(t *testing.T) {
	if _, err := ParseMap(nil); err != nil {
		t.Fatalf("empty map: %v", err)
	}
}

func TestParseMapRefusals(t *testing.T) {
	cases := map[string]string{
		"unknown role":   "roles: [customer]\nroutes:\n  - {route: GET /a, allow: [admin], source: x, status: approved}\n",
		"no source":      "roles: [customer]\nroutes:\n  - {route: GET /a, allow: [customer], status: approved}\n",
		"bad status":     "roles: [customer]\nroutes:\n  - {route: GET /a, allow: [customer], source: x, status: maybe}\n",
		"no allow":       "roles: [customer]\nroutes:\n  - {route: GET /a, source: x, status: approved}\n",
		"no method":      "roles: [customer]\nroutes:\n  - {route: /a, allow: [customer], source: x, status: approved}\n",
		"query":          "roles: [customer]\nroutes:\n  - {route: 'GET /a?x=1', allow: [customer], source: x, status: approved}\n",
		"same shape":     "roles: [customer]\nroutes:\n  - {route: 'GET /a/{id}', allow: [customer], source: x, status: approved}\n  - {route: 'GET /a/{other}', allow: [customer], source: x, status: approved}\n",
		"reserved role":  "roles: [owner]\nroutes: []\n",
		"duplicate role": "roles: [customer, Customer]\nroutes: []\n",
		"unknown field":  "roles: [customer]\nrules: []\n",
		"bad param":      "roles: [customer]\nroutes:\n  - {route: 'GET /a/{id', allow: [customer], source: x, status: approved}\n",
	}
	for name, doc := range cases {
		_, err := ParseMap([]byte(doc))
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if !strings.HasPrefix(err.Error(), "berries: ") {
			t.Errorf("%s: error %q does not start with the package name", name, err)
		}
	}
}

func TestAllows(t *testing.T) {
	m, err := ParseMap([]byte(sampleMap))
	if err != nil {
		t.Fatal(err)
	}
	inv, _ := m.Match("GET", "/invoices/1")
	if !inv.Allows("alice", []string{"customer"}, "alice") {
		t.Error("owner denied")
	}
	if inv.Allows("bob", []string{"customer"}, "alice") {
		t.Error("another customer allowed on an owner rule")
	}
	if !inv.Allows("root", []string{" Admin "}, "alice") {
		t.Error("admin tag not matched regardless of case and space")
	}
	me, _ := m.Match("GET", "/me")
	if !me.Allows("bob", nil, "alice") {
		t.Error("any did not allow an untagged user")
	}
}
