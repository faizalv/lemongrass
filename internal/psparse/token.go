// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import (
	"math/big"
	"strconv"
)

// TokenKind is PowerShell's TokenKind; the constants and their values are generated into token_tables.go.
type TokenKind int

// TokenFlags is PowerShell's TokenFlags.
type TokenFlags uint32

// String returns PowerShell's name for the kind, the name the oracle reports.
func (k TokenKind) String() string {
	if k >= 0 && int(k) < len(tokenKindNames) && tokenKindNames[k] != "" {
		return tokenKindNames[k]
	}
	return "TokenKind(" + strconv.Itoa(int(k)) + ")"
}

// Traits returns the flags every token of this kind carries.
func (k TokenKind) Traits() TokenFlags {
	if k < 0 || int(k) >= len(staticTokenFlags) {
		return FlagNone
	}
	return staticTokenFlags[k]
}

// HasTrait reports whether every token of this kind carries flag.
func (k TokenKind) HasTrait(flag TokenFlags) bool { return k.Traits()&flag != 0 }

// BinaryPrecedence is the precedence of a binary operator kind.
func (k TokenKind) BinaryPrecedence() int { return int(k.Traits() & FlagBinaryPrecedenceMask) }

// Text is PowerShell's text for the kind ("-and", "foreach", "newline").
func (k TokenKind) Text() string {
	if k < 0 || int(k) >= len(tokenText) {
		return ""
	}
	return tokenText[k]
}

// RedirectionStream is PowerShell's RedirectionStream.
type RedirectionStream int

const (
	StreamAll RedirectionStream = iota
	StreamOutput
	StreamError
	StreamWarning
	StreamVerbose
	StreamDebug
	StreamInformation
)

// Token is every PowerShell token class in one struct; Kind says which of the class-specific fields apply. One
// struct keeps the tokenizer and parser free of type switches over token classes, at the cost of unused fields.
type Token struct {
	Kind   TokenKind
	Flags  TokenFlags
	Extent Extent
	// Text is the token's source text, as Token.Text in PowerShell.
	Text string

	// Number: Value is int32, int64, uint32, uint64, int8, uint8, int16, uint16, float64, Decimal or *big.Int,
	// the .NET type PowerShell picks for the literal.
	Value any

	// Parameter.
	ParameterName string
	UsedColon     bool

	// Variable and SplattedVariable.
	VariablePath *VariablePath

	// StringLiteral, StringExpandable, HereStringLiteral, HereStringExpandable, Generic and Identifier carry their
	// value here (StringToken.Value); Label carries LabelText.
	StringValue string
	// StringExpandable and HereStringExpandable.
	NestedTokens []*Token
	FormatString string

	// Redirection (merging and file) and RedirectInStd.
	FromStream RedirectionStream
	ToStream   RedirectionStream
	Append     bool
	// Merging is true for a merging redirection such as 2>&1, false for a file redirection.
	Merging bool

	// SkippedCharOffsets marks, for an unscanned subexpression, the offsets the tokenizer skipped (escaped
	// backticks), so a nested scan maps its offsets back onto the input.
	SkippedCharOffsets []bool
}

// HasError reports whether the tokenizer marked the token in error.
func (t *Token) HasError() bool { return t.Flags&FlagTokenInError != 0 }

// setIsCommandArgument mirrors Token.SetIsCommandArgument: a token read as a command argument becomes Generic.
func (t *Token) setIsCommandArgument() {
	if t.Kind != Identifier {
		t.Kind = Generic
	}
}

// Decimal is a .NET decimal: Unscaled divided by ten to the power Scale.
type Decimal struct {
	Unscaled *big.Int
	Scale    uint8
}
