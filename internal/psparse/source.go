// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import "unicode/utf16"

// Extent is a half-open range of UTF-16 code unit offsets into the parsed input. PowerShell strings are UTF-16, so
// offsets in these units are the ones the reference parser reports and the oracle compares.
type Extent struct {
	Start int
	End   int
}

// Empty reports whether the extent covers no input.
func (e Extent) Empty() bool { return e.End <= e.Start }

// source is the input as the tokenizer reads it: UTF-16 code units, the way PowerShell indexes a string.
type source struct {
	text []uint16
}

func newSource(input string) source {
	return source{text: utf16.Encode([]rune(input))}
}

// slice returns the input covered by e as a Go string.
func (s source) slice(e Extent) string {
	start, end := clampExtent(e, len(s.text))
	return string(utf16.Decode(s.text[start:end]))
}

func clampExtent(e Extent, n int) (int, int) {
	start, end := e.Start, e.End
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	if end < start {
		end = start
	}
	return start, end
}

// Limits bound the work one parse may do. Input past either limit is refused with an error, never parsed in part.
type Limits struct {
	// MaxInput is the largest input accepted, in UTF-16 code units.
	MaxInput int
	// MaxDepth is the deepest nesting of statements, expressions and script blocks accepted. PowerShell retries a
	// deep parse on a larger stack; a bound turns hostile nesting into a refusal instead.
	MaxDepth int
}

// DefaultLimits match the script-file cap of the guard's script scan (256 KiB) and allow far deeper nesting than
// any hand-written command needs.
var DefaultLimits = Limits{MaxInput: 256 * 1024, MaxDepth: 200}
