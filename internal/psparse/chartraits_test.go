// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import "testing"

func TestOrdinalEqualFold(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"Switch", "SWITCH", true},
		{"foreach", "ForEach", true},
		{"break", "brea\u212a", false}, // Kelvin sign is not k
		{"switch", "\u017fwitch", false},
		{"if", "\u0131f", false},
		{"\u00e9t\u00e9", "\u00c9T\u00c9", true},
		{"a", "ab", false},
	}
	for _, c := range cases {
		if got := OrdinalEqualFold(c.a, c.b); got != c.want {
			t.Errorf("OrdinalEqualFold(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
	if !ordinalHasPrefixFold("RunAsAdministrator", "runas") || ordinalHasPrefixFold("run", "runas") {
		t.Error("ordinalHasPrefixFold")
	}
	if OrdinalUpper("get-ChildItem") != "GET-CHILDITEM" {
		t.Error("OrdinalUpper")
	}
}

func TestWTF8RoundTrip(t *testing.T) {
	for _, u := range [][]uint16{
		{'a', 0xd800, 'b'},
		{0xdc00},
		{0xd83d, 0xde00},
		{0xd800, 0xd800, 0xdc00},
	} {
		got := stringToUTF16(utf16ToString(u))
		if len(got) != len(u) {
			t.Fatalf("round trip %x = %x", u, got)
		}
		for i := range u {
			if got[i] != u[i] {
				t.Fatalf("round trip %x = %x", u, got)
			}
		}
	}
	if s := utf16ToString([]uint16{0xd83d, 0xde00}); s != "\U0001F600" {
		t.Errorf("pair decoded as %q", s)
	}
}

func TestCharTraits(t *testing.T) {
	if !isWhitespace(0xa0) || isWhitespace('\n') || !isWhitespace(0x3000) || isWhitespace(0x100) {
		t.Error("isWhitespace")
	}
	if !isDash(0x2013) || !isSingleQuote(0x2019) || !isDoubleQuote(0x201e) {
		t.Error("special quotes and dashes")
	}
	if isIdentifierStart(0xd800) || !isIdentifierStart(0x00e9) || !isVariableStart('?') {
		t.Error("identifier traits")
	}
	if !forceStartNewTokenAfterNumber('?', true) || forceStartNewTokenAfterNumber('?', false) {
		t.Error("ternary chars")
	}
}
