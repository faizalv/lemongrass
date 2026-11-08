// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import "testing"

func TestSourceLinesAndColumns(t *testing.T) {
	// \r\n is one line end; \n then \r are two.
	src := newSource("ab\r\nc\r\nd\n\re")
	if want := []int{0, 4, 7, 9, 10}; !equalInts(src.lineStarts, want) {
		t.Fatalf("lineStarts = %v, want %v", src.lineStarts, want)
	}
	src = newSource("ab\r\nc\rd\ne")
	cases := []struct{ offset, line, column int }{
		{0, 1, 1}, {1, 1, 2}, {2, 1, 3}, {3, 1, 4}, // \r and \n of \r\n stay on line 1
		{4, 2, 1}, {5, 2, 2}, // c, then \r
		{6, 3, 1}, {7, 3, 2}, // d, then \n
		{8, 4, 1}, {9, 4, 2}, // e, then end of input
		{-1, 0, 0},
	}
	for _, c := range cases {
		if c.offset >= 0 {
			if got := src.line(c.offset); got != c.line {
				t.Errorf("line(%d) = %d, want %d", c.offset, got, c.line)
			}
		}
		if got := src.column(c.offset); got != c.column {
			t.Errorf("column(%d) = %d, want %d", c.offset, got, c.column)
		}
	}
	if got := src.startColumn(Extent{4, 9}); got != 1 {
		t.Errorf("startColumn = %d, want 1", got)
	}
	if got := src.endColumn(Extent{4, 9}); got != 2 {
		t.Errorf("endColumn = %d, want 2", got)
	}
	if src.startColumn(emptyExtent) != 0 || src.endColumn(emptyExtent) != 0 {
		t.Error("emptyExtent columns are not 0")
	}
}

func TestSourceUTF16(t *testing.T) {
	// U+1F600 is two UTF-16 units, so b sits at offset 3, as PowerShell indexes the string.
	src := newSource("a\U0001F600b")
	if len(src.text) != 4 {
		t.Fatalf("len = %d, want 4", len(src.text))
	}
	if got := src.slice(Extent{1, 3}); got != "\U0001F600" {
		t.Errorf("slice(1,3) = %q", got)
	}
	if got := src.column(3); got != 4 {
		t.Errorf("column(3) = %d, want 4", got)
	}
	// A WTF-8 lone surrogate is one unit and survives a round trip.
	lone := "x\xed\xa0\x80y"
	src = newSource(lone)
	if len(src.text) != 3 || src.text[1] != 0xd800 {
		t.Fatalf("text = %x", src.text)
	}
	if got := src.slice(Extent{0, 3}); got != lone {
		t.Errorf("slice = %q, want %q", got, lone)
	}
}

func TestResultText(t *testing.T) {
	r := &Result{src: newSource("Get-Item 'é\U0001F600'")}
	cases := []struct {
		ext  Extent
		want string
	}{
		{Extent{0, 8}, "Get-Item"},
		{Extent{9, 14}, "'é\U0001F600'"},
		{Extent{9, 13}, "'é\U0001F600"},
		{Extent{9, 100}, "'é\U0001F600'"},
		{emptyExtent, ""},
		{Extent{5, 5}, ""},
	}
	for _, c := range cases {
		if got := r.Text(c.ext); got != c.want {
			t.Errorf("Text(%v) = %q, want %q", c.ext, got, c.want)
		}
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
