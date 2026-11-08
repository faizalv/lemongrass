// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import "testing"

func TestFormatMessage(t *testing.T) {
	cases := []struct {
		template string
		args     []any
		want     string
	}{
		{"Unexpected token '{0}' in expression or statement.", []any{"x"}, "Unexpected token 'x' in expression or statement."},
		{"{1} then {0}, {0}", []any{"a", "b"}, "b then a, a"},
		{"@{{ModuleName=\"{0}\"}}", []any{"M"}, "@{ModuleName=\"M\"}"},
		{"[{0,4}] [{0,-4}] [{0:N0}]", []any{7}, "[   7] [7   ] [7]"},
		{"{2} {x} {0", []any{"a"}, "{2} {x} {0"},
	}
	for _, c := range cases {
		if got := formatMessage(c.template, c.args); got != c.want {
			t.Errorf("formatMessage(%q) = %q, want %q", c.template, got, c.want)
		}
	}
}

func TestNewParseError(t *testing.T) {
	// Without args SaveError leaves the template as is, doubled braces included.
	e := newParseError(Extent{1, 2}, "IncompleteDollarVariableReference", true)
	if e.Message != parserStrings["IncompleteDollarVariableReference"] || !e.IncompleteInput || e.Extent != (Extent{1, 2}) {
		t.Errorf("no-arg error = %+v", e)
	}
	e = newParseError(Extent{}, "MissingEndCurlyBrace", false, "unused")
	if e.Message != parserStrings["MissingEndCurlyBrace"] {
		t.Errorf("message = %q", e.Message)
	}
	for _, id := range []string{ErrInputTooLarge, ErrNestingTooDeep, ErrDynamicKeywords, ErrUnreachable, ErrRequiresNotSupported} {
		e := newParseError(Extent{}, id, false)
		if e.Message == "" || e.Message == id {
			t.Errorf("%s has no message", id)
		}
		if _, clash := parserStrings[id]; clash {
			t.Errorf("%s collides with a PowerShell error ID", id)
		}
	}
	if got := newParseError(Extent{}, "NoSuchID", false).Message; got != "NoSuchID" {
		t.Errorf("unknown ID message = %q", got)
	}
}
