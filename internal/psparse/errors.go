// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// ParseError is the first error PowerShell's parser would report for the input. ID is the resource key PowerShell
// uses as its ErrorId, so the oracle compares IDs directly.
type ParseError struct {
	Extent          Extent
	ID              string
	Message         string
	IncompleteInput bool
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("psparse: %s at offset %d: %s", e.ID, e.Extent.Start, e.Message)
}

// Error IDs this port reports that PowerShell does not, for input it refuses on purpose.
const (
	ErrInputTooLarge   = "LemongrassInputTooLarge"
	ErrNestingTooDeep  = "LemongrassNestingTooDeep"
	ErrDynamicKeywords = "LemongrassDynamicKeywordNotSupported"
	// ErrUnreachable is reported where C# builds an ErrorStatementAst without having reported an error, a branch
	// PowerShell never reaches with valid input; reporting keeps the parser from returning a partial tree.
	ErrUnreachable = "LemongrassUnreachableErrorStatement"
	// ErrRequiresNotSupported is reported on a #requires line until GetScriptRequirements is ported.
	ErrRequiresNotSupported = "LemongrassRequiresNotSupported"
)

// lemongrassStrings are the messages of the Lemongrass error IDs, which ParserStrings.resx does not hold.
var lemongrassStrings = map[string]string{
	ErrInputTooLarge:        "The input is larger than the PowerShell parser accepts.",
	ErrNestingTooDeep:       "The input nests deeper than the PowerShell parser accepts.",
	ErrDynamicKeywords:      "DSC configurations and dynamic keywords are not supported.",
	ErrUnreachable:          "The parser reached a statement PowerShell treats as an error without a message.",
	ErrRequiresNotSupported: "#requires is not supported yet.",
}

// abortParse unwinds the parser from the first error to Parse, which recovers it. Nothing after the first error is
// needed, since PowerShell runs nothing from input with a parse error.
type abortParse struct{ err *ParseError }

// newParseError formats the message from PowerShell's template for id, as SaveError does: the template is formatted
// only when args are given.
func newParseError(ext Extent, id string, incomplete bool, args ...any) *ParseError {
	msg, ok := parserStrings[id]
	if !ok {
		msg, ok = lemongrassStrings[id]
	}
	if !ok {
		msg = id
	}
	if len(args) > 0 {
		msg = formatMessage(msg, args)
	}
	return &ParseError{Extent: ext, ID: id, Message: msg, IncompleteInput: incomplete}
}

// formatMessage is the part of .NET's string.Format the resx templates use: {n}, {n,align} and {n:format} take
// argument n (the format string is ignored), and {{ and }} are literal braces. A malformed item is kept as written,
// where .NET would throw.
func formatMessage(template string, args []any) string {
	var b strings.Builder
	for i := 0; i < len(template); i++ {
		c := template[i]
		if c == '}' {
			if i+1 < len(template) && template[i+1] == '}' {
				i++
			}
			b.WriteByte('}')
			continue
		}
		if c != '{' {
			b.WriteByte(c)
			continue
		}
		if i+1 < len(template) && template[i+1] == '{' {
			b.WriteByte('{')
			i++
			continue
		}
		end := strings.IndexByte(template[i:], '}')
		if end < 0 {
			b.WriteString(template[i:])
			break
		}
		item := template[i+1 : i+end]
		if text, ok := formatItem(item, args); ok {
			b.WriteString(text)
		} else {
			b.WriteString(template[i : i+end+1])
		}
		i += end
	}
	return b.String()
}

// formatItem renders one {n,align:format} item, padding to align in UTF-16 units as .NET does (left-aligned when negative).
func formatItem(item string, args []any) (string, bool) {
	spec := item
	if k := strings.IndexByte(spec, ':'); k >= 0 {
		spec = spec[:k]
	}
	align := 0
	if k := strings.IndexByte(spec, ','); k >= 0 {
		a, err := strconv.Atoi(strings.TrimSpace(spec[k+1:]))
		if err != nil {
			return "", false
		}
		align, spec = a, spec[:k]
	}
	n, err := strconv.Atoi(strings.TrimSpace(spec))
	if err != nil || n < 0 || n >= len(args) {
		return "", false
	}
	text := fmt.Sprint(args[n])
	width := align
	if width < 0 {
		width = -width
	}
	if pad := width - len(utf16.Encode([]rune(text))); pad > 0 {
		if align > 0 {
			text = strings.Repeat(" ", pad) + text
		} else {
			text += strings.Repeat(" ", pad)
		}
	}
	return text, true
}
