// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"errors"
	"testing"
)

var powerShellRefusalRules = []string{"powershell-construct", "name-redefinition", "drive-definition", "history-replay"}

func TestPowerShellRefusalRulesLocked(t *testing.T) {
	known := knownRules()
	for _, id := range powerShellRefusalRules {
		info, ok := known[id]
		if !ok {
			t.Fatalf("rule %s is not in the catalog", id)
		}
		if !info.Locked || info.Mode != "deny" {
			t.Errorf("rule %s = %+v, want locked deny", id, info)
		}
		if err := (Policy{Rules: map[string]string{id: overrideAllow}}).Validate(); !errors.Is(err, ErrInvalidPolicy) {
			t.Errorf("an override of %s validated: %v", id, err)
		}
	}
}

func TestScriptRefusalsReported(t *testing.T) {
	for _, id := range powerShellRefusalRules {
		in := bashInput(t, "")
		c := checker{in: in, policy: Policy{Rules: map[string]string{id: overrideAllow}}, home: ResolveDir(in.Home)}
		script := shellScript{
			Calls:    []shellCall{{Name: "ls"}},
			Refusals: []Verdict{ruleVerdict(id, "Matched: a test construct.")},
		}
		got := c.pick(c.scriptVerdicts(script))
		if got == nil || got.RuleID != id || got.Mode != ModeDeny || got.Detail != "Matched: a test construct." {
			t.Errorf("refusal %s picked %+v", id, got)
		}
	}
	in := bashInput(t, "")
	c := checker{in: in, home: ResolveDir(in.Home)}
	if got := c.pick(c.scriptVerdicts(shellScript{Calls: []shellCall{{Name: "ls"}}})); got != nil {
		t.Errorf("a script without refusals picked %+v", got)
	}
}
