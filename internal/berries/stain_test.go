// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package berries

import (
	"testing"
	"time"
)

func runWith(id int, tested []Finding, found ...Finding) *Run {
	r := &Run{ID: id, At: time.Unix(int64(id), 0), Findings: found}
	for _, f := range append(tested, found...) {
		r.Tested = append(r.Tested, f.Scope())
	}
	return r
}

func TestMergeLifecycle(t *testing.T) {
	leak := Finding{Check: CheckHorizontal, Route: "GET /invoices/{id}", Owner: "alice", User: "bob", State: StateConfirmed}
	other := Finding{Check: CheckVertical, Route: "GET /admin", Owner: "root", User: "bob", State: StateConfirmed}

	stains := Merge(nil, runWith(1, nil, leak, other))
	if len(stains) != 2 || stains[0].Status != StatusNew {
		t.Fatalf("run 1: %+v", stains)
	}
	id := stains[0].ID

	stains = Merge(stains, runWith(2, nil, leak, other))
	if stains[0].Status != StatusOpen || stains[0].ID != id {
		t.Fatalf("run 2: %+v", stains[0])
	}

	// leak is tested and gone; other was not tested at all, so it stays open.
	stains = Merge(stains, runWith(3, []Finding{leak}))
	if stains[0].Status != StatusFixed || stains[0].FixedRun != 3 || stains[1].Status != StatusOpen {
		t.Fatalf("run 3: %+v", stains)
	}

	stains = Merge(stains, runWith(4, nil, leak))
	if stains[0].Status != StatusRegressed {
		t.Fatalf("run 4: %+v", stains[0])
	}

	stains[1].Status = StatusAccepted
	stains = Merge(stains, runWith(5, nil, other))
	if stains[1].Status != StatusAccepted {
		t.Fatalf("a run changed an accepted stain: %+v", stains[1])
	}
	stains = Merge(stains, runWith(6, []Finding{other}))
	if stains[1].Status != StatusAccepted {
		t.Fatalf("a run fixed an accepted stain: %+v", stains[1])
	}
}

func TestStoreSeedsReplaceTheSameCall(t *testing.T) {
	st := Store{Dir: t.TempDir()}
	s := Seed{Rester: "shop", Environment: "staging", User: "alice", Method: "GET", Path: "/a", Status: 200, RecordedAt: time.Unix(1, 0)}
	first, err := st.SaveSeed(s)
	if err != nil {
		t.Fatal(err)
	}
	s.Status, s.RecordedAt = 201, time.Unix(2, 0)
	second, _ := st.SaveSeed(s)
	s.Path = "/b"
	st.SaveSeed(s)
	seeds, err := st.Seeds()
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || len(seeds) != 2 || seeds[0].Status != 201 {
		t.Fatalf("ids %s %s, seeds %+v", first.ID, second.ID, seeds)
	}
	if st.NextRunID() != 1 {
		t.Fatal("first run id is not 1")
	}
	if err := st.SaveRun(&Run{ID: 1}); err != nil || st.NextRunID() != 2 {
		t.Fatalf("after one run: %v, next %d", err, st.NextRunID())
	}
}
