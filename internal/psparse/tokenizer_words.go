// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// isASCIIVarNameChar is the explicit character list ScanVariable keeps in an unbraced name.
func isASCIIVarNameChar(c uint16) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '?'
}

// scanVariable is ScanVariable; the $ or @ has been consumed. In command mode a variable followed by a character
// that does not end a token is rescanned as a generic token, so $a+b is one argument.
func (t *tokenizer) scanVariable(splatted, inStringExpandable bool) *Token {
	errorStartPosition := t.currentIndex
	var sb []uint16
	c := t.getChar()

	if c == '{' {
	braced:
		for {
			c = t.getChar()
			switch c {
			case '}':
				break braced
			case '`':
				c1 := t.getChar()
				if c1 == 0 && t.atEOF() {
					t.ungetChar()
					break braced
				}
				var surrogate uint16
				c, surrogate = t.backtick(c1)
				if surrogate != invalidChar {
					sb = append(sb, c, surrogate)
					continue
				}
			case '"', charQuoteDoubleLeft, charQuoteDoubleRight, charQuoteLowDoubleLft:
				if inStringExpandable {
					c1 := t.getChar()
					if c1 == 0 && t.atEOF() {
						t.ungetChar()
						break braced
					}
					if isDoubleQuote(c1) {
						c = c1
					} else {
						t.ungetChar()
					}
				}
			case '{':
				t.reportErrorAt(t.currentIndex, "OpenBraceNeedsToBeBackTickedInVariableName")
			case 0:
				if t.atEOF() {
					t.ungetChar()
					break braced
				}
			}
			sb = append(sb, c)
		}

		name := utf16ToString(sb)
		if c != '}' {
			t.reportIncompleteInputAt(errorStartPosition, "IncompleteDollarVariableReference")
		}
		if len(sb) == 0 {
			if c == '}' {
				t.reportErrorAt(t.currentIndex-1, "EmptyVariableReference")
			}
			name = ":Error:"
		}
		if t.inCommandMode() {
			// A '.' or '[' after the name is an operator; anything else that does not end a token makes the
			// whole word a generic token, rescanned from the start.
			if c1 := t.peekChar(); !forceStartNewToken(c1) && c1 != '.' && c1 != '[' {
				t.currentIndex = t.tokenStart
				return t.scanGenericToken(nil)
			}
		}
		path := NewVariablePath(name)
		if path.UnqualifiedPath() == "" {
			t.parser.reportError(t.newScriptExtent(t.tokenStart, t.currentIndex), "InvalidBracedVariableReference")
		}
		return t.newVariableToken(path, false)
	}

	if !isVariableStart(c) {
		t.ungetChar()
		return t.scanGenericToken([]uint16{'$'})
	}

	sb = append(sb, c)
	if !(c == '$' || c == '?' || c == '^') {
	scan:
		for {
			c = t.getChar()
			switch {
			case isASCIIVarNameChar(c):
				sb = append(sb, c)
			case c == ':':
				if t.peekChar() == ':' {
					// $a::b is static member access.
					t.ungetChar()
					break scan
				}
				sb = append(sb, c)
			case c == 0, c == '\t', c == '\r', c == '\n', c == ' ', c == '&', c == '(', c == ')', c == ',', c == ';',
				c == '{', c == '}', c == '|', c == '.', c == '[', c == '=':
				t.ungetChar()
				break scan
			case netIsLetterOrDigit(c):
				sb = append(sb, c)
			case t.inCommandMode() && !forceStartNewToken(c):
				t.currentIndex = t.tokenStart
				return t.scanGenericToken(nil)
			default:
				t.ungetChar()
				break scan
			}
		}
	} else if t.inCommandMode() && !forceStartNewToken(t.peekChar()) {
		t.currentIndex = t.tokenStart
		return t.scanGenericToken(nil)
	}

	path := NewVariablePath(utf16ToString(sb))
	if path.UnqualifiedPath() == "" {
		id := "InvalidVariableReference"
		if path.IsDriveQualified() {
			id = "InvalidVariableReferenceWithDrive"
		}
		t.parser.reportError(t.newScriptExtent(t.tokenStart, t.currentIndex), id)
	}
	return t.newVariableToken(path, splatted)
}

// scanParameter is ScanParameter: a parameter or, in expression mode, an operator; the dash has been consumed.
func (t *tokenizer) scanParameter() *Token {
	var sb []uint16
	sawColonAtEnd := false
scan:
	for {
		c := t.getChar()
		if isWhitespace(c) {
			t.ungetChar()
			break
		}
		switch {
		case c == '{' || c == '}' || c == '(' || c == ')' || c == ';' || c == ',' || c == '|' || c == '&' ||
			c == '.' || c == '[' || c == '\r' || c == '\n' || c == 0:
			t.ungetChar()
			break scan
		case c == ':':
			sawColonAtEnd = true
			if !t.inCommandMode() {
				t.ungetChar()
			}
			break scan
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
			sb = append(sb, c)
		case isSingleQuote(c) || isDoubleQuote(c):
			if t.inCommandMode() {
				// Quotes are never part of a parameter: the whole word is an argument.
				t.ungetChar()
				word := append([]uint16{t.scriptAt(t.tokenStart)}, sb...)
				return t.scanGenericToken(word)
			}
			t.ungetChar()
			break scan
		default:
			if t.inCommandMode() {
				sb = append(sb, c)
			} else {
				t.ungetChar()
				break scan
			}
		}
	}
	str := utf16ToString(sb)
	if t.inExpressionMode() {
		if kind, ok := operatorTable[OrdinalUpper(str)]; ok {
			return t.newToken(kind)
		}
	}
	if len(sb) == 0 {
		return t.newToken(Minus)
	}
	return t.newParameterToken(str, sawColonAtEnd)
}

// checkOperatorInCommandMode is CheckOperatorInCommandMode(char, TokenKind): in command mode an operator glued
// to more text is a generic token.
func (t *tokenizer) checkOperatorInCommandMode(c uint16, kind TokenKind) *Token {
	if t.inCommandMode() && !forceStartNewToken(t.peekChar()) {
		return t.scanGenericTokenChar(c)
	}
	return t.newToken(kind)
}

// checkOperatorInCommandMode2 is CheckOperatorInCommandMode(char, char, TokenKind).
func (t *tokenizer) checkOperatorInCommandMode2(c1, c2 uint16, kind TokenKind) *Token {
	if t.inCommandMode() && !forceStartNewToken(t.peekChar()) {
		return t.scanGenericToken([]uint16{c1, c2})
	}
	return t.newToken(kind)
}

// scanGenericTokenChar is ScanGenericToken(char).
func (t *tokenizer) scanGenericTokenChar(first uint16) *Token {
	return t.scanGenericToken([]uint16{first})
}

// scanGenericTokenChars is ScanGenericToken(char, char): the low surrogate is appended unless it is invalidChar.
func (t *tokenizer) scanGenericTokenChars(first, surrogate uint16) *Token {
	sb := []uint16{first}
	if surrogate != invalidChar {
		sb = append(sb, surrogate)
	}
	return t.scanGenericToken(sb)
}

// scanGenericToken is ScanGenericToken(StringBuilder): a command name or argument, read until a character that
// forces a new token. Quotes, escapes, variables and subexpressions inside it are scanned as in an expandable
// string, and when any are nested the token is expandable.
func (t *tokenizer) scanGenericToken(sb []uint16) *Token {
	nested := []*Token{}
	formatSb := append([]uint16(nil), sb...)
	c := t.getChar()
	for ; !forceStartNewToken(c); c = t.getChar() {
		if c == '`' {
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
		} else if isSingleQuote(c) {
			n := len(sb)
			t.scanStringLiteral(&sb)
			formatSb = append(formatSb, sb[n:]...)
			continue
		} else if isDoubleQuote(c) {
			t.scanStringExpandable(&sb, &formatSb, &nested)
			continue
		} else if c == '$' {
			if t.scanDollarInStringExpandable(&sb, &formatSb, false, &nested) {
				continue
			}
		}
		sb = append(sb, c)
		formatSb = append(formatSb, c)
		if c == '{' || c == '}' {
			formatSb = append(formatSb, c)
		}
	}
	t.ungetChar()
	str := utf16ToString(sb)
	if len(nested) > 0 {
		return t.newGenericExpandableToken(str, utf16ToString(formatSb), nested)
	}
	return t.newGenericToken(str)
}

// scanDot is ScanDot; the first dot has been consumed.
func (t *tokenizer) scanDot() *Token {
	c := t.peekChar()
	if c == '.' {
		t.skipChar()
		c = t.peekChar()
		if t.inCommandMode() && !forceStartNewToken(c) {
			t.ungetChar()
			return t.scanGenericTokenChar('.')
		}
		return t.newToken(DotDot)
	}
	if isDecimalDigit(c) {
		return t.scanNumber('.')
	}
	// .$command, ."command" and .'command' dot-source a command; any other glued text is a command name.
	if t.inCommandMode() && !forceStartNewToken(c) && c != '$' && c != '"' && c != '\'' {
		return t.scanGenericTokenChar('.')
	}
	return t.newToken(Dot)
}

// scanIdentifier is ScanIdentifier: a keyword, a command name or argument, or a member name.
func (t *tokenizer) scanIdentifier(first uint16) *Token {
	sb := []uint16{first}
	var c uint16
	for {
		c = t.getChar()
		if !isIdentifierFollow(c) {
			t.ungetChar()
			break
		}
		sb = append(sb, c)
	}
	if t.inTypeNameMode() {
		return t.scanTypeName()
	}
	if !t.WantSimpleName && t.inCommandMode() && !forceStartNewToken(c) {
		return t.scanGenericToken(sb)
	}
	if !t.WantSimpleName && (t.inCommandMode() || t.inSignatureMode()) {
		if kind, ok := keywordTable[OrdinalUpper(utf16ToString(sb))]; ok {
			if kind != InlineScript || t.InWorkflowContext {
				return t.newToken(kind)
			}
		}
	}
	return t.newToken(Identifier)
}

// scanTypeName is ScanTypeName: the rest of a type name, flagged TypeName.
func (t *tokenizer) scanTypeName() *Token {
	for {
		c := t.getChar()
		switch c {
		case '.', '`', '_', '+', '#', '\\':
			continue
		}
		if netIsLetterOrDigit(c) {
			continue
		}
		t.ungetChar()
		break
	}
	tok := t.newToken(Identifier)
	tok.Flags |= FlagTypeName
	return tok
}

// scanLabel is ScanLabel; the ':' has been consumed in command mode.
func (t *tokenizer) scanLabel() *Token {
	var sb []uint16
	c := t.getChar()
	if !isIdentifierStart(c) {
		sb = append(sb, ':')
		if c == 0 {
			t.ungetChar()
			return t.newGenericToken(utf16ToString(sb))
		}
		t.ungetChar()
		return t.scanGenericToken(sb)
	}
	for isIdentifierFollow(c) {
		sb = append(sb, c)
		c = t.getChar()
	}
	if t.inCommandMode() && !forceStartNewToken(c) {
		word := append([]uint16{':'}, sb...)
		word = append(word, c)
		return t.scanGenericToken(word)
	}
	t.ungetChar()
	return t.newLabelToken(utf16ToString(sb))
}
