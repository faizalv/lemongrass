// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import "strconv"

const maxNumberOfUnicodeHexDigits = 6

// backtick is Backtick: the character an escape stands for, and the low surrogate when it needs two units
// (invalidChar otherwise).
func (t *tokenizer) backtick(c uint16) (uint16, uint16) {
	switch c {
	case '0':
		return 0, invalidChar
	case 'a':
		return '\a', invalidChar
	case 'b':
		return '\b', invalidChar
	case 'e':
		return 0x1b, invalidChar
	case 'f':
		return '\f', invalidChar
	case 'n':
		return '\n', invalidChar
	case 'r':
		return '\r', invalidChar
	case 't':
		return '\t', invalidChar
	case 'u':
		return t.scanUnicodeEscape()
	case 'v':
		return '\v', invalidChar
	}
	return c, invalidChar
}

// scanUnicodeEscape is ScanUnicodeEscape: `u{X} with one to six hex digits.
func (t *tokenizer) scanUnicodeEscape() (uint16, uint16) {
	escSeqStartIndex := t.currentIndex - 2
	c := t.getChar()
	if c != '{' {
		t.ungetChar()
		t.parser.reportError(t.newScriptExtent(escSeqStartIndex, t.currentIndex), "InvalidUnicodeEscapeSequence")
		return invalidChar, invalidChar
	}
	var hex []byte
	for i := 0; i < maxNumberOfUnicodeHexDigits+1; i++ {
		c = t.getChar()
		if c == '}' {
			if i == 0 {
				t.parser.reportError(t.newScriptExtent(escSeqStartIndex, t.currentIndex), "InvalidUnicodeEscapeSequence")
				return invalidChar, invalidChar
			}
			break
		} else if !isHexDigit(c) {
			t.ungetChar()
			if i < maxNumberOfUnicodeHexDigits {
				t.reportErrorAt(t.currentIndex, "InvalidUnicodeEscapeSequence")
			} else {
				t.reportErrorAt(t.currentIndex, "MissingUnicodeEscapeSequenceTerminator")
			}
			return invalidChar, invalidChar
		} else if i == maxNumberOfUnicodeHexDigits {
			t.ungetChar()
			t.reportErrorAt(t.currentIndex, "TooManyDigitsInUnicodeEscapeSequence")
			return invalidChar, invalidChar
		}
		hex = append(hex, byte(c))
	}
	v, _ := strconv.ParseUint(string(hex), 16, 32)
	switch {
	case v <= 0xffff:
		return uint16(v), invalidChar
	case v <= 0x10ffff:
		v -= 0x10000
		return uint16(v/0x400 + 0xd800), uint16(v%0x400 + 0xdc00)
	}
	t.parser.reportError(t.newScriptExtent(escSeqStartIndex+3, t.currentIndex-1), "InvalidUnicodeEscapeSequenceValue")
	return invalidChar, invalidChar
}

// getVerbatimCommandArgument is GetVerbatimCommandArgument, the text after --% up to a newline, an unquoted pipe
// or an unquoted &&, ignoring every other PowerShell rule.
func (t *tokenizer) getVerbatimCommandArgument() *Token {
	t.skipWhiteSpace()
	t.tokenStart = t.currentIndex
	inQuotes := false
	for {
		c := t.getChar()
		if c == '\r' || c == '\n' || (c == 0 && t.atEOF()) {
			t.ungetChar()
			break
		}
		if isDoubleQuote(c) {
			inQuotes = !inQuotes
			continue
		}
		if !inQuotes && (c == '|' || (c == '&' && !t.atEOF() && t.peekChar() == '&')) {
			t.ungetChar()
			break
		}
	}
	return t.newStringLiteralToken(t.src.slice(t.currentExtent()), Generic, FlagNone)
}

// scanStringLiteral is ScanStringLiteral(StringBuilder): the body of a single-quoted string, ” standing for '.
func (t *tokenizer) scanStringLiteral(sb *[]uint16) TokenFlags {
	errorIndex := t.currentIndex - 1
	c := t.getChar()
	for c != 0 || !t.atEOF() {
		if isSingleQuote(c) {
			if !isSingleQuote(t.peekChar()) {
				break
			}
			c = t.getChar()
		}
		*sb = append(*sb, c)
		c = t.getChar()
	}
	if c == 0 && t.atEOF() {
		t.ungetChar()
		t.reportIncompleteInputAt(errorIndex, "TerminatorExpectedAtEndOfString", "'")
		return FlagTokenInError
	}
	return FlagNone
}

// scanStringLiteralToken is ScanStringLiteral().
func (t *tokenizer) scanStringLiteralToken() *Token {
	var sb []uint16
	flags := t.scanStringLiteral(&sb)
	return t.newStringLiteralToken(utf16ToString(sb), StringLiteral, flags)
}

// scanSubExpression is ScanSubExpression: the raw text of $(...) inside an expandable string, scanned later by
// a nested scan. A doubled quote collapses to one, and its first half is recorded in skippedCharOffsets so the
// nested scan's extents still land on the input.
func (t *tokenizer) scanSubExpression(hereString bool) *Token {
	t.tokenStart = t.currentIndex - 2
	sb := []uint16{'$', '('}
	parenCount := 1
	flags := FlagNone
	var skipped []int
	for scanning := true; scanning; {
		c := t.getChar()
		switch {
		case c == '(':
			sb = append(sb, c)
			parenCount++
		case c == ')':
			sb = append(sb, c)
			parenCount--
			if parenCount == 0 {
				scanning = false
			}
		case c == '`' || c == '"' || c == charQuoteDoubleLeft || c == charQuoteDoubleRight || c == charQuoteLowDoubleLft:
			c1 := t.peekChar()
			if !hereString && isDoubleQuote(c1) {
				t.skipChar()
				sb = append(sb, c1)
				skipped = append(skipped, t.currentIndex-2+t.nestedTokensAdjustment)
			} else {
				sb = append(sb, c)
			}
		case c == 0 && t.atEOF():
			t.ungetChar()
			t.reportIncompleteInputAt(t.tokenStart, "IncompleteDollarSubexpressionReference")
			flags = FlagTokenInError
			scanning = false
		default:
			sb = append(sb, c)
		}
	}
	skippedOffsets := t.skippedCharOffsets
	if len(skipped) > 0 {
		skippedOffsets = make([]bool, skipped[len(skipped)-1]+1)
		for _, i := range skipped {
			if i >= 0 {
				skippedOffsets[i] = true
			}
		}
	}
	// Built with new in C#, not through SaveToken: the token is never recorded or remembered as first or last.
	tok := t.makeToken(t.currentExtent(), StringLiteral, ClassUnscannedSubExprToken, flags)
	tok.StringValue = utf16ToString(sb)
	tok.script = sb
	tok.SkippedCharOffsets = skippedOffsets
	return tok
}

// scanStringExpandable is ScanStringExpandable(StringBuilder, StringBuilder, List): the body of a double-quoted
// string. sb collects the value and formatSb the format string, where each nested variable or subexpression is
// {n} and literal braces are doubled.
func (t *tokenizer) scanStringExpandable(sb, formatSb *[]uint16, nested *[]*Token) TokenFlags {
	errorIndex := t.currentIndex - 1
	c := t.getChar()
	for ; c != 0 || !t.atEOF(); c = t.getChar() {
		if isDoubleQuote(c) {
			if !isDoubleQuote(t.peekChar()) {
				break
			}
			c = t.getChar()
		} else if c == '$' {
			if t.scanDollarInStringExpandable(sb, formatSb, false, nested) {
				continue
			}
		} else if c == '`' {
			if c1 := t.peekChar(); c1 != 0 {
				t.skipChar()
				var surrogate uint16
				c, surrogate = t.backtick(c1)
				if surrogate != invalidChar {
					*sb = append(*sb, c, surrogate)
					*formatSb = append(*formatSb, c, surrogate)
					continue
				}
			}
		}
		if c == '{' || c == '}' {
			*formatSb = append(*formatSb, c)
		}
		*sb = append(*sb, c)
		*formatSb = append(*formatSb, c)
	}
	if c == 0 && t.atEOF() {
		t.ungetChar()
		t.reportIncompleteInputAt(errorIndex, "TerminatorExpectedAtEndOfString", "\"")
		return FlagTokenInError
	}
	return FlagNone
}

// scanDollarInStringExpandable is ScanDollarInStringExpandable: a variable or subexpression inside an
// expandable string or generic token, scanned in expression mode and not saved. It reports whether one was found.
func (t *tokenizer) scanDollarInStringExpandable(sb, formatSb *[]uint16, hereString bool, nested *[]*Token) bool {
	dollarIndex := t.currentIndex - 1
	c1 := t.peekChar()
	saveTokenStart := t.tokenStart
	oldMode := t.Mode
	oldTokenList := t.tokenList
	var nestedToken *Token
	func() {
		defer func() {
			t.tokenList = oldTokenList
			t.tokenStart = saveTokenStart
			t.Mode = oldMode
		}()
		t.tokenList = nil
		t.Mode = ModeExpression
		if c1 == '(' {
			t.skipChar()
			nestedToken = t.scanSubExpression(hereString)
		} else if isVariableStart(c1) || c1 == '{' {
			t.tokenStart = t.currentIndex - 1
			nestedToken = t.scanVariable(false, true)
		}
	}()
	if nestedToken == nil {
		return false
	}
	*sb = append(*sb, t.script[dollarIndex:min(t.currentIndex, len(t.script))]...)
	*formatSb = append(*formatSb, '{')
	*formatSb = append(*formatSb, stringToUTF16(strconv.Itoa(len(*nested)))...)
	*formatSb = append(*formatSb, '}')
	*nested = append(*nested, nestedToken)
	return true
}

// scanStringExpandableToken is ScanStringExpandable().
func (t *tokenizer) scanStringExpandableToken() *Token {
	var sb, formatSb []uint16
	nested := []*Token{}
	flags := t.scanStringExpandable(&sb, &formatSb, &nested)
	return t.newStringExpandableToken(utf16ToString(sb), utf16ToString(formatSb), StringExpandable, nested, flags)
}

// scanAfterHereStringHeader is ScanAfterHereStringHeader: only whitespace may follow @' or @" before the newline.
func (t *tokenizer) scanAfterHereStringHeader(quote uint16) bool {
	headerOffset := t.currentIndex - 2
	var c uint16
	for {
		c = t.getChar()
		if !isWhitespace(c) {
			break
		}
	}
	if c == '\r' {
		t.normalizeCRLF(c)
		return true
	}
	if c == '\n' {
		return true
	}
	if c == 0 && t.atEOF() {
		t.ungetChar()
		t.reportIncompleteInputAt(headerOffset, "TerminatorExpectedAtEndOfString", string(rune(quote))+"@")
		return false
	}
	t.ungetChar()
	t.reportErrorAt(t.currentIndex, "UnexpectedCharactersAfterHereStringHeader")
	return false
}

// scanPossibleHereStringFooter is ScanPossibleHereStringFooter, at the start of a line: the footer, or
// whitespace before a footer-like '@ that falseFooterOffset remembers for a better error.
func (t *tokenizer) scanPossibleHereStringFooter(test func(uint16) bool, appendChar func(uint16), falseFooterOffset *int) bool {
	c := t.getChar()
	if test(c) && t.peekChar() == '@' {
		t.skipChar()
		return true
	}
	for isWhitespace(c) {
		appendChar(c)
		c = t.getChar()
	}
	if c == '\r' || c == '\n' || (c == 0 && t.atEOF()) {
		t.ungetChar()
		return false
	}
	if test(c) && t.peekChar() == '@' {
		appendChar(c)
		if *falseFooterOffset == -1 {
			*falseFooterOffset = t.currentIndex - 1
		}
		appendChar(t.getChar())
	} else {
		t.ungetChar()
	}
	return false
}

// scanHereStringLiteral is ScanHereStringLiteral; @' has been consumed.
func (t *tokenizer) scanHereStringLiteral() *Token {
	headerOffset := t.currentIndex - 2
	falseFooterOffset := -1
	if !t.scanAfterHereStringHeader('\'') {
		return t.newStringLiteralToken("", HereStringLiteral, FlagTokenInError)
	}
	flags := FlagNone
	var sb []uint16
	appendChar := func(c uint16) { sb = append(sb, c) }
	if !t.scanPossibleHereStringFooter(isSingleQuote, appendChar, &falseFooterOffset) {
		for {
			c := t.getChar()
			if c == '\r' || c == '\n' {
				length := len(sb)
				sb = append(sb, c)
				if c == '\r' && t.peekChar() == '\n' {
					t.skipChar()
					sb = append(sb, '\n')
				}
				if t.scanPossibleHereStringFooter(isSingleQuote, appendChar, &falseFooterOffset) {
					sb = sb[:length]
					break
				}
			} else if c != 0 || !t.atEOF() {
				sb = append(sb, c)
			} else {
				t.ungetChar()
				if falseFooterOffset != -1 {
					t.reportIncompleteInputAt(falseFooterOffset, "WhitespaceBeforeHereStringFooter")
				} else {
					t.reportIncompleteInputAt(headerOffset, "TerminatorExpectedAtEndOfString", "'@")
				}
				flags = FlagTokenInError
				break
			}
		}
	}
	return t.newStringLiteralToken(utf16ToString(sb), HereStringLiteral, flags)
}

// scanHereStringExpandable is ScanHereStringExpandable; @" has been consumed.
func (t *tokenizer) scanHereStringExpandable() *Token {
	headerOffset := t.currentIndex - 2
	if !t.scanAfterHereStringHeader('"') {
		return t.newStringExpandableToken("", "", HereStringExpandable, nil, FlagTokenInError)
	}
	flags := FlagNone
	nested := []*Token{}
	falseFooterOffset := -1
	var sb, formatSb []uint16
	appendChar := func(c uint16) {
		sb = append(sb, c)
		formatSb = append(formatSb, c)
	}
	if !t.scanPossibleHereStringFooter(isDoubleQuote, appendChar, &falseFooterOffset) {
		for {
			c := t.getChar()
			if c == '\r' || c == '\n' {
				length, formatLength := len(sb), len(formatSb)
				sb = append(sb, c)
				formatSb = append(formatSb, c)
				if c == '\r' && t.peekChar() == '\n' {
					t.skipChar()
					sb = append(sb, '\n')
					formatSb = append(formatSb, '\n')
				}
				if t.scanPossibleHereStringFooter(isDoubleQuote, appendChar, &falseFooterOffset) {
					sb = sb[:length]
					formatSb = formatSb[:formatLength]
					break
				}
				continue
			}
			if c == '$' {
				if t.scanDollarInStringExpandable(&sb, &formatSb, true, &nested) {
					continue
				}
			} else if c == '`' {
				if c1 := t.peekChar(); c1 != 0 {
					t.skipChar()
					var surrogate uint16
					c, surrogate = t.backtick(c1)
					if surrogate != invalidChar {
						sb = append(sb, c, surrogate)
						formatSb = append(formatSb, c, surrogate)
						continue
					}
				}
			}
			if c == '{' || c == '}' {
				formatSb = append(formatSb, c)
			}
			if c != 0 || !t.atEOF() {
				sb = append(sb, c)
				formatSb = append(formatSb, c)
			} else {
				t.ungetChar()
				if falseFooterOffset != -1 {
					t.reportIncompleteInputAt(falseFooterOffset, "WhitespaceBeforeHereStringFooter")
				} else {
					t.reportIncompleteInputAt(headerOffset, "TerminatorExpectedAtEndOfString", "\"@")
				}
				flags = FlagTokenInError
				break
			}
		}
	}
	return t.newStringExpandableToken(utf16ToString(sb), utf16ToString(formatSb), HereStringExpandable, nested, flags)
}
