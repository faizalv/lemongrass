// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import "unicode"

// Signature lookalike detection (ScanToEndOfCommentLine): a line comment within an edit distance of 10 of the
// signature block header ends the valid script, since readers stop at a signature and code hidden after a fake
// one would go unread.
const (
	beginSignatureTextNoSpace      = "sig#beginsignatureblock\n"
	beginTokenSimilarityThreshold  = 10
	beginTokenSimilarityUpperBound = 34
	beginTokenSimilarityLowerBound = 14
)

// scanToEndOfCommentLine is ScanToEndOfCommentLine: it scans a line comment and reports whether it looks like
// a signature block header and whether it starts with "requires".
func (t *tokenizer) scanToEndOfCommentLine() (sawBeginSig, matchedRequires bool) {
	// The comment's non-whitespace characters, the newline or end-of-input character included, as C# collects
	// them.
	var commentLine []uint16
	requiresMatchState := 0
	for {
		c := t.getChar()
		if !isWhitespace(c) {
			commentLine = append(commentLine, c)
		}
		switch c {
		case 'e', 'E':
			if requiresMatchState == 1 || requiresMatchState == 6 {
				requiresMatchState++
			} else {
				requiresMatchState = -1
			}
			continue
		case 'i', 'I':
			if requiresMatchState == 4 {
				requiresMatchState++
			} else {
				requiresMatchState = -1
			}
			continue
		case 'q', 'Q':
			if requiresMatchState == 2 {
				requiresMatchState++
			} else {
				requiresMatchState = -1
			}
			continue
		case 'r', 'R':
			if requiresMatchState == 0 || requiresMatchState == 5 {
				requiresMatchState++
			} else {
				requiresMatchState = -1
			}
			continue
		case 's', 'S':
			if requiresMatchState == 7 {
				matchedRequires = true
			} else {
				requiresMatchState = -1
			}
			continue
		case 'u', 'U':
			if requiresMatchState == 3 {
				requiresMatchState++
			} else {
				requiresMatchState = -1
			}
			continue
		case 0:
			if !t.atEOF() {
				requiresMatchState = -1
				continue
			}
		case '\r', '\n':
		default:
			requiresMatchState = -1
			continue
		}
		t.ungetChar()
		if len(commentLine) > beginTokenSimilarityUpperBound || len(commentLine) < beginTokenSimilarityLowerBound {
			return false, matchedRequires
		}
		for i, u := range commentLine {
			commentLine[i] = lowerInvariantUnit(u)
		}
		return getStringSimilarity(commentLine, stringToUTF16(beginSignatureTextNoSpace)) < beginTokenSimilarityThreshold,
			matchedRequires
	}
}

// lowerInvariantUnit is char.ToLowerInvariant for one UTF-16 unit.
func lowerInvariantUnit(c uint16) uint16 {
	if c < 128 {
		if c >= 'A' && c <= 'Z' {
			return c + ('a' - 'A')
		}
		return c
	}
	if isSurrogate(c) {
		return c
	}
	l := unicode.ToLower(rune(c))
	if l > 0xffff {
		return c
	}
	return uint16(l)
}

// getStringSimilarity is GetStringSimilarity, the Levenshtein distance.
func getStringSimilarity(first, second []uint16) int {
	prev := make([]int, len(second)+1)
	cur := make([]int, len(second)+1)
	for col := range prev {
		prev[col] = col
	}
	for row := 1; row <= len(first); row++ {
		cur[0] = row
		for col := 1; col <= len(second); col++ {
			if first[row-1] == second[col-1] {
				cur[col] = prev[col-1]
			} else {
				cur[col] = min(prev[col]+1, cur[col-1]+1, prev[col-1]+1)
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(second)]
}

// scanLineComment is ScanLineComment. A #requires comment is remembered only outside nested scans.
func (t *tokenizer) scanLineComment() {
	sawBeginSig, matchedRequires := t.scanToEndOfCommentLine()
	tok := t.newCommentToken()
	if sawBeginSig {
		e := t.currentExtent()
		t.beginSignatureExtent = &e
	} else if matchedRequires && t.nestedTokensAdjustment == 0 {
		if t.requiresTokens == nil {
			t.requiresTokens = &[]*Token{}
		}
		*t.requiresTokens = append(*t.requiresTokens, tok)
	}
}

// scanBlockComment is ScanBlockComment; the opening <# has been consumed.
func (t *tokenizer) scanBlockComment() {
	errorIndex := t.currentIndex - 2
	for {
		c := t.getChar()
		if c == '#' && t.peekChar() == '>' {
			t.skipChar()
			break
		}
		if c == '\r' {
			t.normalizeCRLF(c)
		} else if c == 0 && t.atEOF() {
			t.ungetChar()
			t.reportIncompleteInputAt(errorIndex, "MissingTerminatorMultiLineComment")
			break
		}
	}
	t.newCommentToken()
}
