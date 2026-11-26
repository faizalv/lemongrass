// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package berries

import (
	"sort"
	"time"
)

// Stain statuses across runs. Accepted is set only by the human in the Berries pane and is never changed by a run.
const (
	StatusNew       = "new"
	StatusOpen      = "open"
	StatusFixed     = "fixed"
	StatusRegressed = "regressed"
	StatusAccepted  = "accepted"
)

// Stain is a finding tracked across runs. Its id is derived from its key, so the same stain keeps its id.
type Stain struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Scope     string    `json:"scope"`
	Status    string    `json:"status"`
	Finding   Finding   `json:"finding"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	FirstRun  int       `json:"first_run"`
	LastRun   int       `json:"last_run"`
	FixedRun  int       `json:"fixed_run,omitempty"`
}

// Open reports whether a stain still needs attention.
func (s Stain) Open() bool {
	return s.Status == StatusNew || s.Status == StatusOpen || s.Status == StatusRegressed
}

// Merge folds a run into the stain list. A finding seen before keeps its id and moves to open, or to regressed when
// it had been fixed; an open stain whose scope the run tested without finding it again becomes fixed; a stain the
// run did not test is left as it was, since not testing proves nothing.
func Merge(stains []Stain, run *Run) []Stain {
	byKey := map[string]int{}
	for i, s := range stains {
		byKey[s.Key] = i
	}
	found := map[string]bool{}
	for _, f := range run.Findings {
		key := f.Key()
		found[key] = true
		if i, ok := byKey[key]; ok {
			s := &stains[i]
			switch s.Status {
			case StatusFixed:
				s.Status = StatusRegressed
			case StatusNew:
				s.Status = StatusOpen
			}
			s.Finding, s.LastSeen, s.LastRun = f, run.At, run.ID
			continue
		}
		stains = append(stains, Stain{ID: shortHash("b", key), Key: key, Scope: f.Scope(), Status: StatusNew, Finding: f,
			FirstSeen: run.At, LastSeen: run.At, FirstRun: run.ID, LastRun: run.ID})
		byKey[key] = len(stains) - 1
	}
	tested := map[string]bool{}
	for _, t := range run.Tested {
		tested[t] = true
	}
	for i := range stains {
		s := &stains[i]
		if s.Open() && !found[s.Key] && tested[s.Scope] {
			s.Status, s.FixedRun = StatusFixed, run.ID
		}
	}
	sort.SliceStable(stains, func(i, j int) bool { return stains[i].FirstSeen.Before(stains[j].FirstSeen) })
	return stains
}
