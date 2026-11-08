// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import "sort"

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
	// lineStarts holds the offset of each line's first unit, as the tokenizer's LineStartMap: \r\n, \r and \n
	// each end a line.
	lineStarts []int
}

// newSource reads input as .NET string units; a WTF-8 encoded lone surrogate is read back as that unit.
func newSource(input string) source {
	text := stringToUTF16(input)
	starts := []int{0}
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\r':
			if i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
			starts = append(starts, i+1)
		case '\n':
			starts = append(starts, i+1)
		}
	}
	return source{text: text, lineStarts: starts}
}

// line is PositionHelper.LineFromOffset: the 1-based line holding offset.
func (s source) line(offset int) int {
	return sort.Search(len(s.lineStarts), func(i int) bool { return s.lineStarts[i] > offset })
}

// column is PositionHelper.ColumnFromOffset: the 1-based column of offset on its line. EmptyScriptExtent's
// columns are 0, so a negative offset yields 0.
func (s source) column(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset - s.lineStarts[s.line(offset)-1] + 1
}

// startColumn and endColumn are IScriptExtent.StartColumnNumber and EndColumnNumber.
func (s source) startColumn(e Extent) int { return s.column(e.Start) }
func (s source) endColumn(e Extent) int   { return s.column(e.End) }

// slice returns the input covered by e as a Go string, a lone surrogate kept as WTF-8.
func (s source) slice(e Extent) string {
	start, end := clampExtent(e, len(s.text))
	return utf16ToString(s.text[start:end])
}

func clampExtent(e Extent, n int) (int, int) {
	start, end := e.Start, e.End
	if start < 0 {
		start = 0
	}
	if start > n {
		// The EOF token's extent sits one past the end of input.
		start = n
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
