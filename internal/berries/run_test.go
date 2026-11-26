// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package berries

import (
	"errors"
	"testing"
)

// fakeAPI answers by user and target, with safe default headers so passive checks stay quiet unless a case wants them.
type fakeAPI struct {
	answers map[string]Response
	calls   []string
}

func (f *fakeAPI) Do(user, method, target string, body []byte, contentType string, headers []Header) (Response, error) {
	f.calls = append(f.calls, user+" "+method+" "+target)
	r, ok := f.answers[user+" "+target]
	if !ok {
		return Response{}, errors.New("no answer")
	}
	if r.Headers == nil {
		r.Headers = map[string][]string{"Cache-Control": {"no-store"}, "X-Content-Type-Options": {"nosniff"}}
	}
	return r, nil
}

var invoice = map[string]any{"id": float64(1), "owner": "alice", "total": float64(10)}

func testUsers() []User {
	return []User{{Name: "alice", Tags: []string{"customer"}}, {Name: "bob", Tags: []string{"customer"}}, {Name: "root", Tags: []string{"admin"}}, {Name: "viewer", Tags: []string{"viewer"}}}
}

const runMap = `
roles: [customer, admin, viewer]
routes:
  - {route: "GET /invoices/{id}", allow: [owner, admin], source: PRD, status: approved}
  - {route: GET /admin/stats, allow: [admin], source: PRD, status: approved}
  - {route: POST /invoices, allow: [customer], source: PRD, status: approved}
`

func mustMap(t *testing.T, doc string) *Map {
	t.Helper()
	m, err := ParseMap([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func outcomes(run *Run) map[string]string {
	out := map[string]string{}
	for _, r := range run.Rows {
		out[r.SeedID+" "+r.User] = r.Outcome
	}
	return out
}

func TestExecuteClassifiesEveryPair(t *testing.T) {
	api := &fakeAPI{answers: map[string]Response{
		"alice /invoices/1":   {Status: 200, Body: invoice},
		"bob /invoices/1":     {Status: 200, Body: invoice},                              // same content: confirmed, horizontal
		"viewer /invoices/1":  {Status: 200, Body: map[string]any{"id": float64(1)}},     // other content: open proof gap, vertical
		"root /invoices/1":    {Status: 403, Body: map[string]any{"error": "forbidden"}}, // allowed but refused: over restricted
		"root /admin/stats":   {Status: 200, Body: map[string]any{"users": float64(4)}},  // control
		"alice /admin/stats":  {Status: 403, Body: "forbidden"},                          // ruled out by status
		"bob /admin/stats":    {Status: 200, Body: []any{}},                              // ruled out by empty body
		"viewer /admin/stats": {Status: 404, Body: nil},
	}}
	seeds := []Seed{
		{ID: "s1", User: "alice", Method: "GET", Path: "/invoices/1", Target: "/invoices/1"},
		{ID: "s2", User: "root", Method: "GET", Path: "/admin/stats", Target: "/admin/stats"},
		{ID: "s3", User: "alice", Method: "POST", Path: "/invoices", Target: "/invoices"},
		{ID: "s4", User: "alice", Method: "GET", Path: "/unknown", Target: "/unknown"},
		{ID: "s5", User: "ghost", Method: "GET", Path: "/invoices/2", Target: "/invoices/2"},
	}
	run := Execute(RunInput{Rester: "shop", Environment: "staging", Users: testUsers(), Map: mustMap(t, runMap), Seeds: seeds, Requester: api})

	got := outcomes(run)
	want := map[string]string{
		"s1 alice": OutcomeAllowed, "s1 bob": StateConfirmed, "s1 viewer": StateOpenProofGap, "s1 root": OutcomeOverRestricted,
		"s2 root": OutcomeAllowed, "s2 alice": StateRuledOut, "s2 bob": StateRuledOut, "s2 viewer": StateRuledOut,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %q, want %q", k, got[k], w)
		}
	}
	if run.SkipWrites != 1 || run.SkipUsers != 1 || len(run.Unmapped) != 1 || run.Replayed != 2 {
		t.Errorf("counts: writes %d users %d unmapped %v replayed %d", run.SkipWrites, run.SkipUsers, run.Unmapped, run.Replayed)
	}
	checks := map[string]string{}
	for _, f := range run.Findings {
		checks[f.User] = f.Check + " " + f.State
	}
	if checks["bob"] != CheckHorizontal+" "+StateConfirmed || checks["viewer"] != CheckVertical+" "+StateOpenProofGap || len(run.Findings) != 2 {
		t.Errorf("findings = %+v", run.Findings)
	}
	for _, c := range api.calls {
		if c == "alice POST /invoices" {
			t.Error("a write was replayed")
		}
	}
}

func TestOwnerOutsideTheRuleIsAStainFromTheSeedAlone(t *testing.T) {
	api := &fakeAPI{answers: map[string]Response{
		"alice /admin/stats": {Status: 200, Body: map[string]any{"users": float64(4)}},
		"root /admin/stats":  {Status: 200, Body: map[string]any{"users": float64(4)}},
	}}
	run := Execute(RunInput{Users: []User{{Name: "alice", Tags: []string{"customer"}}, {Name: "root", Tags: []string{"admin"}}},
		Map: mustMap(t, runMap), Requester: api,
		Seeds: []Seed{{ID: "s1", User: "alice", Method: "GET", Path: "/admin/stats", Target: "/admin/stats"}}})
	if len(run.Findings) != 1 || run.Findings[0].User != "alice" || run.Findings[0].Check != CheckVertical || run.Findings[0].State != StateConfirmed {
		t.Fatalf("findings = %+v", run.Findings)
	}
}

func TestFailedControlStopsTheSeed(t *testing.T) {
	api := &fakeAPI{answers: map[string]Response{"alice /invoices/1": {Status: 404}}}
	run := Execute(RunInput{Users: testUsers(), Map: mustMap(t, runMap), Requester: api,
		Seeds: []Seed{{ID: "s1", User: "alice", Method: "GET", Path: "/invoices/1", Target: "/invoices/1"}}})
	if len(run.Rows) != 1 || run.Rows[0].Outcome != OutcomeControlFailed || len(api.calls) != 1 {
		t.Fatalf("rows = %+v, calls = %v", run.Rows, api.calls)
	}
}

func TestOnlySeedAndUserNarrowTheRun(t *testing.T) {
	api := &fakeAPI{answers: map[string]Response{
		"alice /invoices/1": {Status: 200, Body: invoice},
		"bob /invoices/1":   {Status: 403},
	}}
	run := Execute(RunInput{Users: testUsers(), Map: mustMap(t, runMap), Requester: api, OnlySeed: "s1", OnlyUser: "bob",
		Seeds: []Seed{{ID: "s1", User: "alice", Method: "GET", Path: "/invoices/1", Target: "/invoices/1"},
			{ID: "s2", User: "root", Method: "GET", Path: "/admin/stats", Target: "/admin/stats"}}})
	if len(api.calls) != 2 {
		t.Fatalf("calls = %v, want the control and bob only", api.calls)
	}
	if len(run.Findings) != 0 || len(run.Tested) == 0 {
		t.Fatalf("findings %+v tested %v", run.Findings, run.Tested)
	}
}

func TestDigestIgnoresKeyOrderAndEmpty(t *testing.T) {
	a := map[string]any{"a": float64(1), "b": "x"}
	b := map[string]any{"b": "x", "a": float64(1)}
	if Digest(a) != Digest(b) {
		t.Error("key order changed the digest")
	}
	for _, body := range []any{nil, "  ", []any{}, map[string]any{}} {
		if !Empty(body) {
			t.Errorf("Empty(%#v) = false", body)
		}
	}
	if Empty("x") || Empty([]any{1}) {
		t.Error("content counted as empty")
	}
}
