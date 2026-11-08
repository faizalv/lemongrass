// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import (
	"fmt"
	"strings"
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
)

// abortParse unwinds the parser from the first error to Parse, which recovers it. Nothing after the first error is
// needed, since PowerShell runs nothing from input with a parse error.
type abortParse struct{ err *ParseError }

// newParseError formats the message from PowerShell's template for id, filling {0}, {1}, ... from args.
func newParseError(ext Extent, id string, incomplete bool, args ...any) *ParseError {
	msg, ok := parserStrings[id]
	if !ok {
		msg = id
	}
	for i, a := range args {
		msg = strings.ReplaceAll(msg, fmt.Sprintf("{%d}", i), fmt.Sprint(a))
	}
	return &ParseError{Extent: ext, ID: id, Message: msg, IncompleteInput: incomplete}
}
