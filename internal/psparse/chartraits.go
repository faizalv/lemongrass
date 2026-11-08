// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import "unicode"

// Special characters PowerShell treats like their ASCII forms (SpecialChars).
const (
	charNoBreakSpace      = 0x00a0
	charNextLine          = 0x0085
	charEnDash            = 0x2013
	charEmDash            = 0x2014
	charHorizontalBar     = 0x2015
	charQuoteSingleLeft   = 0x2018
	charQuoteSingleRight  = 0x2019
	charQuoteSingleBase   = 0x201a
	charQuoteReversed     = 0x201b
	charQuoteDoubleLeft   = 0x201c
	charQuoteDoubleRight  = 0x201d
	charQuoteLowDoubleLft = 0x201e
)

// charTraits is CharTraits.
type charTraits uint16

const (
	ctNone                               charTraits = 0
	ctIdentifierStart                    charTraits = 0x0002
	ctMultiplierStart                    charTraits = 0x0004
	ctTypeSuffix                         charTraits = 0x0008
	ctWhitespace                         charTraits = 0x0010
	ctNewline                            charTraits = 0x0020
	ctHexDigit                           charTraits = 0x0040
	ctDigit                              charTraits = 0x0080
	ctVarNameFirst                       charTraits = 0x0100
	ctForceStartNewToken                 charTraits = 0x0200
	ctForceStartNewAssemblyNameSpecToken charTraits = 0x0400
	ctForceStartNewTokenAfterNumber      charTraits = 0x0800
)

// asciiTraits is CharExtensions.s_traits, the traits of the first 128 characters.
var asciiTraits = func() [128]charTraits {
	var t [128]charTraits
	const fsAll = ctForceStartNewToken | ctForceStartNewAssemblyNameSpecToken
	t[0x0] = fsAll
	t['\t'] = ctWhitespace | fsAll
	t['\n'] = ctNewline | fsAll
	t['\v'] = ctWhitespace | fsAll
	t['\f'] = ctWhitespace | fsAll
	t['\r'] = ctNewline | fsAll
	t[' '] = ctWhitespace | fsAll
	for _, c := range "!#%*+-./<>" {
		t[c] = ctForceStartNewTokenAfterNumber
	}
	t['$'] = ctVarNameFirst
	t['&'] = ctForceStartNewToken
	t['('] = ctForceStartNewToken
	t[')'] = ctForceStartNewToken
	t[','] = fsAll
	for c := '0'; c <= '9'; c++ {
		t[c] = ctDigit | ctHexDigit | ctVarNameFirst
	}
	t[':'] = ctVarNameFirst
	t[';'] = ctForceStartNewToken
	t['='] = ctForceStartNewAssemblyNameSpecToken | ctForceStartNewTokenAfterNumber
	t['?'] = ctVarNameFirst
	for c := 'A'; c <= 'Z'; c++ {
		lower := c + 'a' - 'A'
		v := ctIdentifierStart | ctVarNameFirst
		if c <= 'F' {
			v |= ctHexDigit
		}
		switch c {
		case 'G', 'K', 'M', 'P', 'T':
			v |= ctMultiplierStart
		case 'D', 'L', 'N', 'S', 'U', 'Y':
			v |= ctTypeSuffix
		}
		t[c] = v
		t[lower] = v
	}
	t[']'] = ctForceStartNewAssemblyNameSpecToken | ctForceStartNewTokenAfterNumber
	t['^'] = ctVarNameFirst
	t['_'] = ctIdentifierStart | ctVarNameFirst
	t['{'] = ctForceStartNewToken
	t['|'] = ctForceStartNewToken
	t['}'] = ctForceStartNewToken
	return t
}()

func isSurrogate(c uint16) bool { return c >= 0xd800 && c <= 0xdfff }

// netClass is the part of a .NET UnicodeCategory the tokenizer asks about.
type netClass uint8

const (
	netOther     netClass = iota
	netLetter             // Lu, Ll, Lt, Lm, Lo
	netDigit              // Nd
	netSeparator          // Zs, Zl, Zp
)

// netClassOverrides holds the BMP code units whose .NET 10 category class differs from Go's unicode tables, which
// lag behind .NET's Unicode version. A token boundary decided by the older table would split a name PowerShell
// keeps whole. Generated from pwsh's [char]::GetUnicodeCategory dump and held to it by a parity test.
var netClassOverrides = map[uint16]netClass{}

// netClassOf classifies a .NET char; a surrogate half is no letter, digit or separator, as in .NET.
func netClassOf(c uint16) netClass {
	if isSurrogate(c) {
		return netOther
	}
	if v, ok := netClassOverrides[c]; ok {
		return v
	}
	r := rune(c)
	switch {
	case unicode.IsLetter(r):
		return netLetter
	case unicode.Is(unicode.Nd, r):
		return netDigit
	case unicode.In(r, unicode.Zs, unicode.Zl, unicode.Zp):
		return netSeparator
	}
	return netOther
}

// netIsLetter is char.IsLetter.
func netIsLetter(c uint16) bool { return netClassOf(c) == netLetter }

// netIsDigit is char.IsDigit (category Nd only).
func netIsDigit(c uint16) bool { return netClassOf(c) == netDigit }

// netIsLetterOrDigit is char.IsLetterOrDigit.
func netIsLetterOrDigit(c uint16) bool {
	k := netClassOf(c)
	return k == netLetter || k == netDigit
}

// netIsSeparator is char.IsSeparator.
func netIsSeparator(c uint16) bool { return netClassOf(c) == netSeparator }

// netIsWhiteSpace is char.IsWhiteSpace: separators plus the C0 and Latin-1 spacing controls.
func netIsWhiteSpace(c uint16) bool {
	switch {
	case c >= '\t' && c <= '\r', c == ' ', c == charNextLine, c == charNoBreakSpace:
		return true
	case c < 0x100:
		return false
	}
	return netIsSeparator(c)
}

// isWhitespace is CharExtensions.IsWhitespace; newlines are not whitespace.
func isWhitespace(c uint16) bool {
	if c < 128 {
		return asciiTraits[c]&ctWhitespace != 0
	}
	if c <= 256 {
		return c == charNoBreakSpace || c == charNextLine
	}
	return netIsSeparator(c)
}

// isDash is IsDash.
func isDash(c uint16) bool {
	return c == '-' || c == charEnDash || c == charEmDash || c == charHorizontalBar
}

// isSingleQuote is IsSingleQuote.
func isSingleQuote(c uint16) bool {
	return c == '\'' || c == charQuoteSingleLeft || c == charQuoteSingleRight || c == charQuoteSingleBase ||
		c == charQuoteReversed
}

// isDoubleQuote is IsDoubleQuote.
func isDoubleQuote(c uint16) bool {
	return c == '"' || c == charQuoteDoubleLeft || c == charQuoteDoubleRight || c == charQuoteLowDoubleLft
}

// isVariableStart is IsVariableStart.
func isVariableStart(c uint16) bool {
	if c < 128 {
		return asciiTraits[c]&ctVarNameFirst != 0
	}
	return netIsLetterOrDigit(c)
}

// isIdentifierStart is IsIdentifierStart.
func isIdentifierStart(c uint16) bool {
	if c < 128 {
		return asciiTraits[c]&ctIdentifierStart != 0
	}
	return netIsLetter(c)
}

// isIdentifierFollow is IsIdentifierFollow.
func isIdentifierFollow(c uint16) bool {
	if c < 128 {
		return asciiTraits[c]&(ctIdentifierStart|ctDigit) != 0
	}
	return netIsLetterOrDigit(c)
}

// isHexDigit is IsHexDigit.
func isHexDigit(c uint16) bool { return c < 128 && asciiTraits[c]&ctHexDigit != 0 }

// isDecimalDigit is IsDecimalDigit.
func isDecimalDigit(c uint16) bool { return c >= '0' && c <= '9' }

// isBinaryDigit is IsBinaryDigit.
func isBinaryDigit(c uint16) bool { return c == '0' || c == '1' }

// isTypeSuffix is IsTypeSuffix.
func isTypeSuffix(c uint16) bool { return c < 128 && asciiTraits[c]&ctTypeSuffix != 0 }

// isMultiplierStart is IsMultiplierStart.
func isMultiplierStart(c uint16) bool { return c < 128 && asciiTraits[c]&ctMultiplierStart != 0 }

// forceStartNewToken is ForceStartNewToken: the character ends a token begun with a letter or digit in any mode, so
// a#b is one token and a{ two.
func forceStartNewToken(c uint16) bool {
	if c < 128 {
		return asciiTraits[c]&ctForceStartNewToken != 0
	}
	return isWhitespace(c)
}

// forceStartNewTokenAfterNumber is ForceStartNewTokenAfterNumber.
func forceStartNewTokenAfterNumber(c uint16, forceEndNumberOnTernaryOperatorChars bool) bool {
	if c < 128 {
		if asciiTraits[c]&ctForceStartNewTokenAfterNumber != 0 {
			return true
		}
		return forceEndNumberOnTernaryOperatorChars && (c == '?' || c == ':')
	}
	return isDash(c)
}

// forceStartNewTokenInAssemblyNameSpec is ForceStartNewTokenInAssemblyNameSpec.
func forceStartNewTokenInAssemblyNameSpec(c uint16) bool {
	if c < 128 {
		return asciiTraits[c]&ctForceStartNewAssemblyNameSpecToken != 0
	}
	return isWhitespace(c)
}

// upperOrdinalUnit is the per-char upper-casing .NET's OrdinalIgnoreCase compares with: the simple uppercase
// mapping, except that U+0131 (dotless i) and U+017F (long s) keep themselves, so no non-ASCII character equals an
// ASCII one. Go's unicode.ToUpper maps those two onto I and S, and strings.EqualFold also folds U+212A onto k, which
// would turn text PowerShell reads as a command name into a keyword or operator.
func upperOrdinalUnit(c uint16) uint16 {
	if c < 128 {
		if c >= 'a' && c <= 'z' {
			return c - ('a' - 'A')
		}
		return c
	}
	if isSurrogate(c) || c == 0x0131 || c == 0x017f {
		return c
	}
	u := unicode.ToUpper(rune(c))
	if u > 0xffff || u < 128 {
		return c
	}
	return uint16(u)
}

// OrdinalUpper upper-cases s the way .NET's OrdinalIgnoreCase compares it, unit by unit, so a map keyed by
// OrdinalUpper behaves like a Dictionary built with StringComparer.OrdinalIgnoreCase.
func OrdinalUpper(s string) string {
	u := stringToUTF16(s)
	for i, c := range u {
		u[i] = upperOrdinalUnit(c)
	}
	return utf16ToString(u)
}

// OrdinalEqualFold is string.Equals(a, b, StringComparison.OrdinalIgnoreCase). Use it, never strings.EqualFold,
// wherever PowerShell compares names case-insensitively.
func OrdinalEqualFold(a, b string) bool {
	ua, ub := stringToUTF16(a), stringToUTF16(b)
	if len(ua) != len(ub) {
		return false
	}
	for i := range ua {
		if upperOrdinalUnit(ua[i]) != upperOrdinalUnit(ub[i]) {
			return false
		}
	}
	return true
}

// ordinalHasPrefixFold is s.StartsWith(prefix, StringComparison.OrdinalIgnoreCase).
func ordinalHasPrefixFold(s, prefix string) bool {
	us, up := stringToUTF16(s), stringToUTF16(prefix)
	if len(up) > len(us) {
		return false
	}
	for i := range up {
		if upperOrdinalUnit(us[i]) != upperOrdinalUnit(up[i]) {
			return false
		}
	}
	return true
}
