// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package berries

import (
	"fmt"
	"sort"
	"strings"
)

// The report functions print what a model reads: counts and ids first, details only on request, the way the web
// engine's results read.

// FormatRun summarizes a run and the stains it touched.
func FormatRun(run *Run, stains []Stain) string {
	var b strings.Builder
	fmt.Fprintf(&b, "berries run %d on %s/%s\n", run.ID, run.Rester, run.Environment)
	fmt.Fprintf(&b, "seeds: %d, replayed %d", run.Seeds, run.Replayed)
	if run.SkipWrites > 0 {
		fmt.Fprintf(&b, ", %d writes skipped (only GET, HEAD and OPTIONS are replayed)", run.SkipWrites)
	}
	if run.SkipUsers > 0 {
		fmt.Fprintf(&b, ", %d skipped because their user is not on this channel", run.SkipUsers)
	}
	if len(run.Unmapped) > 0 {
		fmt.Fprintf(&b, ", %d unmapped (no route in %s)", len(run.Unmapped), MapFile)
	}
	b.WriteString("\n")

	counts := map[string]int{}
	for _, r := range run.Rows {
		counts[r.Outcome]++
	}
	fmt.Fprintf(&b, "pairs: %d tested: %s\n", len(run.Rows), countList(counts,
		OutcomeAllowed, StateRuledOut, OutcomeOverRestricted, StateOpenProofGap, StateConfirmed, OutcomeError, OutcomeControlFailed))

	touched := map[string]bool{}
	for _, f := range run.Findings {
		touched[f.Key()] = true
	}
	statusCounts := map[string]int{}
	var lines []string
	for _, s := range stains {
		inRun := touched[s.Key]
		fixedNow := s.Status == StatusFixed && s.FixedRun == run.ID
		if !inRun && !fixedNow {
			continue
		}
		statusCounts[s.Status]++
		lines = append(lines, "  "+stainLine(s))
	}
	fmt.Fprintf(&b, "stains: %s\n", countList(statusCounts, StatusNew, StatusOpen, StatusRegressed, StatusFixed, StatusAccepted))
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	if len(lines) > 0 {
		b.WriteString("details: lgrass berries stain <id>\n")
	}
	if counts[OutcomeControlFailed] > 0 {
		b.WriteString("a control failed when a seed no longer succeeds as its own user; record it again with lgrass rester\n")
	}
	if len(run.Unmapped) > 0 {
		b.WriteString("unmapped requests are listed by lgrass berries map\n")
	}
	return b.String()
}

func stainLine(s Stain) string {
	f := s.Finding
	return fmt.Sprintf("%s %s %s %s %s: %s", s.ID, s.Status, f.State, f.Check, f.Route, f.Detail)
}

func countList(counts map[string]int, order ...string) string {
	var parts []string
	for _, k := range order {
		if counts[k] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[k], k))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// FormatStains lists stains, open ones only unless all is set.
func FormatStains(stains []Stain, all bool) string {
	var b strings.Builder
	n := 0
	for _, s := range stains {
		if !all && !s.Open() {
			continue
		}
		n++
		b.WriteString(stainLine(s) + "\n")
	}
	if n == 0 {
		if all {
			return "no stains yet\n"
		}
		return "no open stains; lgrass berries stains --all lists fixed and accepted ones too\n"
	}
	return b.String()
}

// FormatStain prints one stain with its evidence pair and the rule it was judged by.
func FormatStain(s Stain) string {
	f := s.Finding
	var b strings.Builder
	fmt.Fprintf(&b, "stain %s: %s, %s, %s\n", s.ID, s.Status, f.State, f.Check)
	fmt.Fprintf(&b, "route: %s on %s/%s, request %s (seed %s)\n", f.Route, f.Rester, f.Environment, f.Path, f.SeedID)
	b.WriteString(f.Detail + "\n")
	if f.Rule != "" {
		fmt.Fprintf(&b, "rule: %s\n", f.Rule)
	}
	if f.Control != nil {
		fmt.Fprintf(&b, "control, as %s: status %d, digest %s\n  %s\n", f.Control.User, f.Control.Status, f.Control.Digest, f.Control.Preview)
	}
	if f.Replay != nil {
		label := "replay"
		if f.Control == nil {
			label = "response"
		}
		fmt.Fprintf(&b, "%s, as %s: status %d, digest %s\n  %s\n", label, f.Replay.User, f.Replay.Status, f.Replay.Digest, f.Replay.Preview)
	}
	fmt.Fprintf(&b, "seen in runs %d to %d", s.FirstRun, s.LastRun)
	if s.FixedRun > 0 {
		fmt.Fprintf(&b, ", fixed in run %d", s.FixedRun)
	}
	b.WriteString("\n")
	if s.Open() {
		b.WriteString("after a fix, confirm it with lgrass berries replay <short-id> " + s.ID + "\n")
	}
	return b.String()
}

// FormatSeeds lists seeds grouped by the route they match, unmapped ones apart.
func FormatSeeds(m *Map, seeds []Seed) string {
	groups := map[string][]Seed{}
	var unmapped []Seed
	for _, s := range seeds {
		if r, ok := m.Match(s.Method, s.Path); ok {
			groups[r.Pattern] = append(groups[r.Pattern], s)
		} else {
			unmapped = append(unmapped, s)
		}
	}
	if len(seeds) == 0 {
		return "no seeds yet; calls made with lgrass rester in this project are recorded as seeds once they succeed\n"
	}
	var b strings.Builder
	for _, r := range m.Routes {
		for i, s := range groups[r.Pattern] {
			if i == 0 {
				b.WriteString(r.Pattern + "\n")
			}
			b.WriteString("  " + seedLine(s) + "\n")
		}
	}
	if len(unmapped) > 0 {
		b.WriteString("unmapped\n")
		for _, s := range unmapped {
			b.WriteString("  " + seedLine(s) + "\n")
		}
	}
	return b.String()
}

func seedLine(s Seed) string {
	target := s.Path
	if s.Query != "" {
		target += "?" + s.Query
	}
	return fmt.Sprintf("%s %s %s as %s on %s/%s, %d", s.ID, s.Method, target, s.User, s.Rester, s.Environment, s.Status)
}

// FormatMap prints the map with each route's seed count, the requests no route covers, and, when the channel's users
// are known, the roles that cannot be tested.
func FormatMap(m *Map, seeds []Seed, users []User) string {
	var b strings.Builder
	counts := map[string]int{}
	unmapped := map[string]bool{}
	for _, s := range seeds {
		if r, ok := m.Match(s.Method, s.Path); ok {
			counts[r.Pattern]++
		} else {
			unmapped[s.Method+" "+s.Path] = true
		}
	}
	fmt.Fprintf(&b, "roles: %s\n", strings.Join(m.Roles, ", "))
	proposed := 0
	for _, r := range m.Routes {
		if r.Status == StatusProposed {
			proposed++
		}
		fmt.Fprintf(&b, "  %s allows %s, %s, %d seeds\n", r.Pattern, strings.Join(r.Allow, ", "), r.Status, counts[r.Pattern])
	}
	if proposed > 0 {
		fmt.Fprintf(&b, "%d rules are still proposed and wait for the human's approval\n", proposed)
	}
	if len(unmapped) > 0 {
		var paths []string
		for p := range unmapped {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		b.WriteString("requests no route covers:\n")
		for _, p := range paths {
			b.WriteString("  " + p + "\n")
		}
	}
	if users != nil {
		for _, role := range m.Roles {
			n := 0
			for _, u := range users {
				for _, t := range u.Tags {
					if normRole(t) == normRole(role) {
						n++
						break
					}
				}
			}
			switch n {
			case 0:
				fmt.Fprintf(&b, "role %s has no user on this channel, so nothing is tested as it\n", role)
			case 1:
				fmt.Fprintf(&b, "role %s has one user on this channel, so one user of that role reading another's objects is not tested\n", role)
			}
		}
	}
	return b.String()
}
