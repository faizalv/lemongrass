// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import "testing"

func TestIsSpecificParameter(t *testing.T) {
	cases := []struct {
		name, parameter string
		want            bool
	}{
		{"re", "regex", true},
		{"REGEX", "regex", true},
		{"regexx", "regex", false},
		{"w", "regex", false},
		// OrdinalIgnoreCase folds no non-ASCII unit onto an ASCII one, unlike unicode.ToUpper.
		{"caſesensitive", "casesensitive", false},
		{"Karam", "parallel", false},
	}
	for _, c := range cases {
		if got := isSpecificParameter(&Token{Kind: Parameter, ParameterName: c.name}, c.parameter); got != c.want {
			t.Errorf("isSpecificParameter(%q, %q) = %v, want %v", c.name, c.parameter, got, c.want)
		}
	}
}
