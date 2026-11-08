// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import (
	"math/big"
	"strconv"
	"unicode/utf8"
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
	Kind TokenKind
	// Class is the C# token class the tokenizer built, since one Kind can come from several classes (a Generic
	// token is a StringLiteralToken or a StringExpandableToken).
	Class  TokenClass
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
	// script is an UnscannedSubExprToken's text as UTF-16, the input a nested scan reads, so it never round-trips
	// through a Go string.
	script []uint16
}

// TokenClass names the C# class of a token, as the oracle reports GetType().Name.
type TokenClass uint8

const (
	ClassToken TokenClass = iota
	ClassNumberToken
	ClassParameterToken
	ClassVariableToken
	ClassStringLiteralToken
	ClassStringExpandableToken
	ClassUnscannedSubExprToken
	ClassLabelToken
	ClassInputRedirectionToken
	ClassMergingRedirectionToken
	ClassFileRedirectionToken
)

var tokenClassNames = [...]string{
	ClassToken:                   "Token",
	ClassNumberToken:             "NumberToken",
	ClassParameterToken:          "ParameterToken",
	ClassVariableToken:           "VariableToken",
	ClassStringLiteralToken:      "StringLiteralToken",
	ClassStringExpandableToken:   "StringExpandableToken",
	ClassUnscannedSubExprToken:   "UnscannedSubExprToken",
	ClassLabelToken:              "LabelToken",
	ClassInputRedirectionToken:   "InputRedirectionToken",
	ClassMergingRedirectionToken: "MergingRedirectionToken",
	ClassFileRedirectionToken:    "FileRedirectionToken",
}

// String is the C# class name.
func (c TokenClass) String() string {
	if int(c) < len(tokenClassNames) {
		return tokenClassNames[c]
	}
	return "TokenClass(" + strconv.Itoa(int(c)) + ")"
}

// isStringToken reports whether the token is a C# StringToken (literal, expandable or unscanned subexpression).
func (t *Token) isStringToken() bool {
	return t.Class == ClassStringLiteralToken || t.Class == ClassStringExpandableToken ||
		t.Class == ClassUnscannedSubExprToken
}

// isStringExpandable reports whether the token is a C# StringExpandableToken.
func (t *Token) isStringExpandable() bool { return t.Class == ClassStringExpandableToken }

// isRedirection reports whether the token is a C# RedirectionToken.
func (t *Token) isRedirection() bool {
	return t.Class == ClassInputRedirectionToken || t.Class == ClassMergingRedirectionToken ||
		t.Class == ClassFileRedirectionToken
}

// utf16ToString converts .NET string units to a Go string. A lone surrogate, which `u{D800} can put into a value,
// is kept as WTF-8 (its three-byte generalized UTF-8 form) rather than replaced by U+FFFD, so the value stays the
// one PowerShell holds; stringToUTF16 reverses it exactly.
func utf16ToString(u []uint16) string {
	b := make([]byte, 0, len(u))
	for i := 0; i < len(u); i++ {
		c := rune(u[i])
		if c >= 0xd800 && c < 0xdc00 && i+1 < len(u) && u[i+1] >= 0xdc00 && u[i+1] <= 0xdfff {
			c = (c-0xd800)<<10 + (rune(u[i+1]) - 0xdc00) + 0x10000
			i++
		}
		if c >= 0xd800 && c <= 0xdfff {
			b = append(b, byte(0xe0|c>>12), byte(0x80|(c>>6)&0x3f), byte(0x80|c&0x3f))
			continue
		}
		b = utf8.AppendRune(b, c)
	}
	return string(b)
}

// stringToUTF16 converts a Go string to .NET string units, reading a WTF-8 encoded surrogate back as that unit.
// Bytes that are not valid UTF-8 or WTF-8 become U+FFFD, as a Go string's invalid bytes do when ranged over.
func stringToUTF16(s string) []uint16 {
	u := make([]uint16, 0, len(s))
	for i := 0; i < len(s); {
		if i+2 < len(s) && s[i] == 0xed && s[i+1] >= 0xa0 && s[i+1] <= 0xbf && s[i+2]&0xc0 == 0x80 {
			u = append(u, uint16(0xd000|uint16(s[i+1]&0x3f)<<6|uint16(s[i+2]&0x3f)))
			i += 3
			continue
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		i += n
		if r >= 0x10000 {
			r -= 0x10000
			u = append(u, uint16(0xd800+(r>>10)), uint16(0xdc00+(r&0x3ff)))
			continue
		}
		u = append(u, uint16(r))
	}
	return u
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
