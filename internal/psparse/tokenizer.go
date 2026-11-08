// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// TokenizerMode is PowerShell's TokenizerMode. The same text tokenizes differently by mode: a bareword is a
// Generic token and -x a Parameter only in Command mode.
type TokenizerMode int

const (
	ModeCommand TokenizerMode = iota
	ModeExpression
	ModeTypeName
	ModeSignature
)

// invalidChar is s_invalidChar, the "no surrogate" marker Backtick returns. It is also a real character
// (U+FFFF), and an escape that yields it is treated as no character, exactly as in C#.
const invalidChar uint16 = 0xffff

// tokenizer is PowerShell's Tokenizer. The parser drives it: it switches Mode, rewinds with resync and rescans,
// so this API follows tokenizer.cs member for member. Each method's doc names the C# member it ports.
//
// Offsets are UTF-16 code units. During a nested scan script is the text of an unscanned subexpression and every
// extent is shifted back onto the input by nestedTokensAdjustment and skippedCharOffsets, as in C#.
type tokenizer struct {
	parser *parser

	Mode                           TokenizerMode
	AllowSignedNumbers             bool
	ForceEndNumberOnTernaryOpChars bool
	WantSimpleName                 bool
	InWorkflowContext              bool

	// tokenList collects every token when non-nil (TokenList); the parser sets it to nil around rescans it does
	// not want recorded, and a nested scan points it at a fresh list.
	tokenList  *[]*Token
	FirstToken *Token
	LastToken  *Token

	// src is the whole input, which extents and token text refer to; script is what is being scanned now.
	src    source
	script []uint16

	tokenStart             int
	currentIndex           int
	nestedTokensAdjustment int
	skippedCharOffsets     []bool
	beginSignatureExtent   *Extent
	// requiresTokens is RequiresTokens: nil until a #requires comment is seen.
	requiresTokens *[]*Token
}

// tokenizerState is TokenizerState, the snapshot a nested scan restores.
type tokenizerState struct {
	nestedTokensAdjustment int
	script                 []uint16
	tokenStart             int
	currentIndex           int
	firstToken             *Token
	lastToken              *Token
	skippedCharOffsets     []bool
	tokenList              *[]*Token
}

func newTokenizer(p *parser) *tokenizer { return &tokenizer{parser: p} }

// initialize is Initialize. The line map lives in source.
func (t *tokenizer) initialize(src source, tokenList *[]*Token) {
	t.src = src
	t.script = src.text
	t.tokenList = tokenList
	t.FirstToken = nil
	t.LastToken = nil
	t.requiresTokens = nil
	t.beginSignatureExtent = nil
	t.currentIndex = 0
	t.Mode = ModeCommand
}

func (t *tokenizer) inCommandMode() bool    { return t.Mode == ModeCommand }
func (t *tokenizer) inExpressionMode() bool { return t.Mode == ModeExpression }
func (t *tokenizer) inTypeNameMode() bool   { return t.Mode == ModeTypeName }
func (t *tokenizer) inSignatureMode() bool  { return t.Mode == ModeSignature }

// startNestedScan is StartNestedScan, for the text of an unscanned subexpression in an expandable string.
func (t *tokenizer) startNestedScan(nested *Token) tokenizerState {
	ts := tokenizerState{
		currentIndex:           t.currentIndex,
		nestedTokensAdjustment: t.nestedTokensAdjustment,
		script:                 t.script,
		tokenStart:             t.tokenStart,
		firstToken:             t.FirstToken,
		lastToken:              t.LastToken,
		skippedCharOffsets:     t.skippedCharOffsets,
		tokenList:              t.tokenList,
	}
	t.currentIndex = 0
	t.nestedTokensAdjustment = nested.Extent.Start
	t.script = nested.script
	t.tokenStart = 0
	t.skippedCharOffsets = nested.SkippedCharOffsets
	if t.tokenList != nil {
		fresh := []*Token{}
		t.tokenList = &fresh
	}
	return ts
}

// finishNestedScan is FinishNestedScan.
func (t *tokenizer) finishNestedScan(ts tokenizerState) {
	t.currentIndex = ts.currentIndex
	t.nestedTokensAdjustment = ts.nestedTokensAdjustment
	t.script = ts.script
	t.tokenStart = ts.tokenStart
	t.FirstToken = ts.firstToken
	t.LastToken = ts.lastToken
	t.skippedCharOffsets = ts.skippedCharOffsets
	t.tokenList = ts.tokenList
}

// getChar is GetChar: it returns 0 past the end and still advances, so ungetChar can unget the end.
func (t *tokenizer) getChar() uint16 {
	cur := t.currentIndex
	t.currentIndex++
	if cur >= len(t.script) || cur < 0 {
		return 0
	}
	return t.script[cur]
}

// ungetChar is UngetChar.
func (t *tokenizer) ungetChar() { t.currentIndex-- }

// peekChar is PeekChar.
func (t *tokenizer) peekChar() uint16 {
	if t.currentIndex >= len(t.script) || t.currentIndex < 0 {
		return 0
	}
	return t.script[t.currentIndex]
}

// skipChar is SkipChar.
func (t *tokenizer) skipChar() { t.currentIndex++ }

// atEOF is AtEof: the last getChar read past the end.
func (t *tokenizer) atEOF() bool { return t.currentIndex > len(t.script) }

// scriptAt reads script[i] like C#'s indexer would, but yields 0 instead of throwing out of range.
func (t *tokenizer) scriptAt(i int) uint16 {
	if i < 0 || i >= len(t.script) {
		return 0
	}
	return t.script[i]
}

// skipNewlines is SkipNewlines. It still creates the newline, semicolon and continuation tokens it skips, since
// the token list holds them.
func (t *tokenizer) skipNewlines(skipSemis bool) {
	for {
		c := t.getChar()
		switch c {
		case ' ', '\t', '\f', '\v', charNoBreakSpace, charNextLine:
			t.skipWhiteSpace()
			continue
		case '\r', '\n':
			t.scanNewline(c)
			continue
		case ';':
			if skipSemis {
				t.scanSemicolon()
				continue
			}
		case '#':
			t.tokenStart = t.currentIndex - 1
			t.scanLineComment()
			continue
		case '<':
			if t.peekChar() == '#' {
				t.tokenStart = t.currentIndex - 1
				t.skipChar()
				t.scanBlockComment()
				continue
			}
		case '`':
			c1 := t.getChar()
			if c1 == '\n' || c1 == '\r' {
				t.scanLineContinuation(c1)
				continue
			}
			if netIsWhiteSpace(c1) {
				t.skipWhiteSpace()
				continue
			}
			t.ungetChar()
		default:
			if isWhitespace(c) {
				t.skipWhiteSpace()
				continue
			}
		}
		t.ungetChar()
		return
	}
}

// skipWhiteSpace is SkipWhiteSpace.
func (t *tokenizer) skipWhiteSpace() {
	for isWhitespace(t.peekChar()) {
		t.skipChar()
	}
}

// scanNewline is ScanNewline.
func (t *tokenizer) scanNewline(c uint16) {
	t.tokenStart = t.currentIndex - 1
	t.normalizeCRLF(c)
	if t.tokenList != nil {
		t.newToken(NewLine)
	}
}

// scanSemicolon is ScanSemicolon.
func (t *tokenizer) scanSemicolon() {
	t.tokenStart = t.currentIndex - 1
	if t.tokenList != nil {
		t.newToken(Semi)
	}
}

// scanLineContinuation is ScanLineContinuation.
func (t *tokenizer) scanLineContinuation(c uint16) {
	t.tokenStart = t.currentIndex - 2
	t.normalizeCRLF(c)
	if t.tokenList != nil {
		t.newToken(LineContinuation)
	}
}

// getRestorePoint is GetRestorePoint.
func (t *tokenizer) getRestorePoint() int {
	t.tokenStart = t.currentIndex
	return t.currentExtent().Start
}

// resyncToken is Resync(Token).
func (t *tokenizer) resyncToken(tok *Token) { t.resync(tok.Extent.Start) }

// resync is Resync(int): the parser backtracks and the tokenizer starts over at start, an input offset.
func (t *tokenizer) resync(start int) {
	adjustment := t.nestedTokensAdjustment
	if t.skippedCharOffsets != nil {
		for i := t.nestedTokensAdjustment; i < start-1 && i < len(t.skippedCharOffsets); i++ {
			if i >= 0 && t.skippedCharOffsets[i] {
				adjustment++
			}
		}
	}
	t.currentIndex = start - adjustment
	if t.currentIndex > len(t.script)+1 {
		t.currentIndex = len(t.script) + 1
	} else if t.currentIndex < 0 {
		t.currentIndex = 0
	}
	if t.FirstToken != nil && t.currentIndex <= t.FirstToken.Extent.Start {
		t.FirstToken = nil
	}
	if t.tokenList != nil && len(*t.tokenList) > 0 {
		removeTokensFromListDuringResync(t.tokenList, start)
	}
	if t.requiresTokens != nil && len(*t.requiresTokens) > 0 {
		removeTokensFromListDuringResync(t.requiresTokens, start)
	}
}

// removeTokensFromListDuringResync is RemoveTokensFromListDuringResync: it drops every saved token from start on,
// keeping a trailing EndOfInput out of the search.
func removeTokensFromListDuringResync(list *[]*Token, start int) {
	l := *list
	removeFrom := 0
	i := len(l) - 1
	if i >= 0 && l[i].Kind == EndOfInput {
		i--
	}
	for ; i >= 0; i-- {
		if l[i].Extent.End <= start {
			removeFrom = i + 1
			break
		}
	}
	for j := removeFrom; j < len(l); j++ {
		l[j] = nil
	}
	*list = l[:removeFrom]
}

// replaceSavedTokens is ReplaceSavedTokens.
func (t *tokenizer) replaceSavedTokens(first, last, replacement *Token) {
	if t.tokenList == nil {
		return
	}
	l := *t.tokenList
	startOffset, endOffset := first.Extent.Start, last.Extent.End
	lastToReplace := -1
	for i := len(l) - 1; i >= 0; i-- {
		if l[i].Extent.End == endOffset {
			lastToReplace = i
			continue
		}
		if l[i].Extent.Start == startOffset {
			if lastToReplace < i {
				// C#'s RemoveRange would throw here; the replacement range is empty, so insert only.
				lastToReplace = i - 1
			}
			out := make([]*Token, 0, len(l)-(lastToReplace-i))
			out = append(out, l[:i]...)
			out = append(out, replacement)
			out = append(out, l[lastToReplace+1:]...)
			*t.tokenList = out
			return
		}
	}
}

// normalizeCRLF is NormalizeCRLF: the \r of \r\n is skipped with its \n.
func (t *tokenizer) normalizeCRLF(c uint16) {
	if c == '\r' && t.peekChar() == '\n' {
		t.skipChar()
	}
}

// checkAstIsBeforeSignature is CheckAstIsBeforeSignature: nothing may follow a comment that looks like a
// signature block, since readers stop there.
func (t *tokenizer) checkAstIsBeforeSignature(n Ast) {
	if t.beginSignatureExtent == nil {
		return
	}
	if t.beginSignatureExtent.Start < n.Extent().Start {
		t.parser.reportError(n.Extent(), "TokenAfterEndOfValidScriptText")
	}
}

// reportErrorAt is ReportError(int, ...): a one-character extent at offset.
func (t *tokenizer) reportErrorAt(offset int, id string, args ...any) {
	t.parser.reportError(t.newScriptExtent(offset, offset+1), id, args...)
}

// reportIncompleteInputAt is ReportIncompleteInput(int, ...): from offset to the current position.
func (t *tokenizer) reportIncompleteInputAt(offset int, id string, args ...any) {
	t.parser.reportIncompleteInput(t.newScriptExtent(offset, t.currentIndex), id, args...)
}

// newScriptExtent is NewScriptExtent.
func (t *tokenizer) newScriptExtent(start, end int) Extent {
	return Extent{Start: start + t.nestedTokensAdjustment, End: end + t.nestedTokensAdjustment}
}

// currentExtent is CurrentExtent: the current token's extent on the input, past any skipped characters.
func (t *tokenizer) currentExtent() Extent {
	start := t.tokenStart + t.nestedTokensAdjustment
	end := t.currentIndex + t.nestedTokensAdjustment
	if t.skippedCharOffsets != nil {
		i := t.nestedTokensAdjustment
		if i < 0 {
			i = 0
		}
		for ; i < start && i < len(t.skippedCharOffsets); i++ {
			if t.skippedCharOffsets[i] {
				start++
				end++
			}
		}
		for ; i < end && i < len(t.skippedCharOffsets); i++ {
			if t.skippedCharOffsets[i] {
				end++
			}
		}
	}
	return Extent{Start: start, End: end}
}

// getScriptExtent is GetScriptExtent, the whole script being scanned.
func (t *tokenizer) getScriptExtent() Extent { return t.newScriptExtent(0, len(t.script)) }

// saveToken is SaveToken: it records the token and tracks the first and last ones for $^ and $$.
func (t *tokenizer) saveToken(tok *Token) *Token {
	if t.tokenList != nil {
		*t.tokenList = append(*t.tokenList, tok)
	}
	switch tok.Kind {
	case NewLine, LineContinuation, Comment, EndOfInput:
	default:
		if t.FirstToken == nil {
			t.FirstToken = tok
		}
		t.LastToken = tok
	}
	return tok
}

// makeToken is the Token constructor: a token carries its kind's static traits on top of flags.
func (t *tokenizer) makeToken(ext Extent, kind TokenKind, class TokenClass, flags TokenFlags) *Token {
	return &Token{Kind: kind, Class: class, Flags: flags | kind.Traits(), Extent: ext, Text: t.extentText(ext)}
}

// extentText is InternalScriptExtent.Text: empty when the extent starts past the input, as EndOfInput does.
func (t *tokenizer) extentText(e Extent) string {
	if e.Start > len(t.src.text) {
		return ""
	}
	return t.src.slice(e)
}

// newCommentToken is NewCommentToken.
func (t *tokenizer) newCommentToken() *Token {
	return t.saveToken(t.makeToken(t.currentExtent(), Comment, ClassToken, FlagNone))
}

// newToken is NewToken.
func (t *tokenizer) newToken(kind TokenKind) *Token {
	return t.saveToken(t.makeToken(t.currentExtent(), kind, ClassToken, FlagNone))
}

// newNumberToken is NewNumberToken.
func (t *tokenizer) newNumberToken(value any) *Token {
	tok := t.makeToken(t.currentExtent(), Number, ClassNumberToken, FlagNone)
	tok.Value = value
	return t.saveToken(tok)
}

// newParameterToken is NewParameterToken.
func (t *tokenizer) newParameterToken(name string, sawColon bool) *Token {
	tok := t.makeToken(t.currentExtent(), Parameter, ClassParameterToken, FlagNone)
	tok.ParameterName = name
	tok.UsedColon = sawColon
	return t.saveToken(tok)
}

// newVariableToken is NewVariableToken.
func (t *tokenizer) newVariableToken(path *VariablePath, splatted bool) *Token {
	kind := Variable
	if splatted {
		kind = SplattedVariable
	}
	tok := t.makeToken(t.currentExtent(), kind, ClassVariableToken, FlagNone)
	tok.VariablePath = path
	return t.saveToken(tok)
}

// newStringLiteralToken is NewStringLiteralToken.
func (t *tokenizer) newStringLiteralToken(value string, kind TokenKind, flags TokenFlags) *Token {
	tok := t.makeToken(t.currentExtent(), kind, ClassStringLiteralToken, flags)
	tok.StringValue = value
	return t.saveToken(tok)
}

// newStringExpandableToken is NewStringExpandableToken. NestedTokens stays nil when there are none, as C#
// leaves it null, and the parser tests it against nil.
func (t *tokenizer) newStringExpandableToken(value, formatString string, kind TokenKind, nested []*Token, flags TokenFlags) *Token {
	if nested != nil && len(nested) == 0 {
		nested = nil
	} else if flags&FlagTokenInError == 0 {
		for _, n := range nested {
			if n.HasError() {
				flags |= FlagTokenInError
				break
			}
		}
	}
	tok := t.makeToken(t.currentExtent(), kind, ClassStringExpandableToken, flags)
	tok.StringValue = value
	tok.FormatString = formatString
	tok.NestedTokens = nested
	return t.saveToken(tok)
}

// newGenericExpandableToken is NewGenericExpandableToken.
func (t *tokenizer) newGenericExpandableToken(value, formatString string, nested []*Token) *Token {
	return t.newStringExpandableToken(value, formatString, Generic, nested, FlagNone)
}

// newGenericToken is NewGenericToken.
func (t *tokenizer) newGenericToken(value string) *Token {
	return t.newStringLiteralToken(value, Generic, FlagNone)
}

// newInputRedirectionToken is NewInputRedirectionToken.
func (t *tokenizer) newInputRedirectionToken() *Token {
	return t.saveToken(t.makeToken(t.currentExtent(), RedirectInStd, ClassInputRedirectionToken, FlagNone))
}

// newFileRedirectionToken is NewFileRedirectionToken: in expression mode 2> is the number 2 followed by >.
func (t *tokenizer) newFileRedirectionToken(from int, appendTo, fromSpecifiedExplicitly bool) *Token {
	if fromSpecifiedExplicitly && t.inExpressionMode() {
		t.ungetChar()
		if appendTo {
			t.ungetChar()
		}
		return t.newNumberToken(int32(from))
	}
	tok := t.makeToken(t.currentExtent(), Redirection, ClassFileRedirectionToken, FlagNone)
	tok.FromStream = RedirectionStream(from)
	tok.Append = appendTo
	return t.saveToken(tok)
}

// newMergingRedirectionToken is NewMergingRedirectionToken.
func (t *tokenizer) newMergingRedirectionToken(from, to int) *Token {
	tok := t.makeToken(t.currentExtent(), Redirection, ClassMergingRedirectionToken, FlagNone)
	tok.FromStream = RedirectionStream(from)
	tok.ToStream = RedirectionStream(to)
	tok.Merging = true
	return t.saveToken(tok)
}

// newLabelToken is NewLabelToken.
func (t *tokenizer) newLabelToken(value string) *Token {
	tok := t.makeToken(t.currentExtent(), Label, ClassLabelToken, FlagNone)
	tok.StringValue = value
	return t.saveToken(tok)
}

// isAtEndOfScript is IsAtEndOfScript. Like C#, it compares the extent's input offset with the length of the text
// being scanned, which in a nested scan is the subexpression's.
func (t *tokenizer) isAtEndOfScript(e Extent, checkCommentsAndWhitespace bool) bool {
	return e.End >= len(t.script) || (checkCommentsAndWhitespace && t.onlyWhitespaceOrCommentsAfterExtent(e))
}

// onlyWhitespaceOrCommentsAfterExtent is OnlyWhitespaceOrCommentsAfterExtent.
func (t *tokenizer) onlyWhitespaceOrCommentsAfterExtent(e Extent) bool {
	for i := max(e.End, 0); i < len(t.script); i++ {
		if t.script[i] == '#' {
			i = t.skipLineCommentAt(i+1) - 1
			continue
		}
		if t.script[i] == '<' && i+1 < len(t.script) && t.script[i+1] == '#' {
			i = t.skipBlockCommentAt(i+2) - 1
			continue
		}
		if !isWhitespace(t.script[i]) {
			return false
		}
	}
	return true
}

// isPipeContinuation is IsPipeContinuation: the first character after a newline, past whitespace and comments,
// is a pipe.
func (t *tokenizer) isPipeContinuation(e Extent) bool {
	return e.End < len(t.script) && t.continuationAfterExtent(e, '|')
}

// continuationAfterExtent is ContinuationAfterExtent.
func (t *tokenizer) continuationAfterExtent(e Extent, continuationChar uint16) bool {
	lastNonWhitespaceIsNewline := true
	i := e.End
	if i < 0 {
		i = 0
	}
	for i < len(t.script)-1 {
		c := t.script[i]
		if isWhitespace(c) {
			i++
			continue
		}
		if c == '\n' {
			if lastNonWhitespaceIsNewline {
				return false
			}
			lastNonWhitespaceIsNewline = true
			i++
			continue
		} else if c == '\r' {
			if lastNonWhitespaceIsNewline {
				return false
			}
			lastNonWhitespaceIsNewline = true
			if t.script[i+1] == '\n' {
				i += 2
			} else {
				i++
			}
			continue
		}
		lastNonWhitespaceIsNewline = false
		if c == '#' {
			i = t.skipLineCommentAt(i + 1)
			continue
		}
		if c == '<' && t.script[i+1] == '#' {
			i = t.skipBlockCommentAt(i + 2)
			continue
		}
		return c == continuationChar
	}
	return t.script[len(t.script)-1] == continuationChar
}

// skipLineCommentAt is SkipLineComment(int).
func (t *tokenizer) skipLineCommentAt(i int) int {
	for ; i < len(t.script); i++ {
		if c := t.script[i]; c == '\r' || c == '\n' {
			break
		}
	}
	return i
}

// skipBlockCommentAt is SkipBlockComment(int).
func (t *tokenizer) skipBlockCommentAt(i int) int {
	for ; i < len(t.script); i++ {
		if t.script[i] == '#' && i+1 < len(t.script) && t.script[i+1] == '>' {
			return i + 2
		}
	}
	return i
}

// getMemberAccessOperator is GetMemberAccessOperator. No whitespace may precede a member access operator, but
// block comments may.
func (t *tokenizer) getMemberAccessOperator(allowLBracket bool) *Token {
	c := t.peekChar()
	for c == '<' {
		t.tokenStart = t.currentIndex
		t.skipChar()
		if t.peekChar() == '#' {
			t.skipChar()
			t.scanBlockComment()
			c = t.peekChar()
		} else {
			t.ungetChar()
			return nil
		}
	}
	switch {
	case c == '.':
		t.tokenStart = t.currentIndex
		t.skipChar()
		c = t.peekChar()
		if c != '.' {
			if t.inCommandMode() && (isWhitespace(c) || c == 0 || c == '\r' || c == '\n') {
				t.ungetChar()
				return nil
			}
			return t.newToken(Dot)
		}
		t.ungetChar()
		return nil
	case c == ':':
		t.tokenStart = t.currentIndex
		t.skipChar()
		c = t.peekChar()
		if c == ':' {
			t.skipChar()
			c = t.peekChar()
			if t.inCommandMode() && (isWhitespace(c) || c == 0 || c == '\r' || c == '\n') {
				t.ungetChar()
				t.ungetChar()
				return nil
			}
			return t.newToken(ColonColon)
		}
		t.ungetChar()
		return nil
	case c == '[' && allowLBracket:
		t.tokenStart = t.currentIndex
		t.skipChar()
		return t.newToken(LBracket)
	case c == '?':
		t.tokenStart = t.currentIndex
		t.skipChar()
		c = t.peekChar()
		if c == '.' {
			t.skipChar()
			return t.newToken(QuestionDot)
		} else if c == '[' && allowLBracket {
			t.skipChar()
			return t.newToken(QuestionLBracket)
		}
		t.ungetChar()
		return nil
	}
	return nil
}

// getInvokeMemberOpenParen is GetInvokeMemberOpenParen.
func (t *tokenizer) getInvokeMemberOpenParen() *Token {
	switch t.peekChar() {
	case '(':
		t.tokenStart = t.currentIndex
		t.skipChar()
		return t.newToken(LParen)
	case '{':
		t.tokenStart = t.currentIndex
		t.skipChar()
		return t.newToken(LCurly)
	}
	return nil
}

// getLBracket is GetLBracket: a '[' or no token, past whitespace and comments.
func (t *tokenizer) getLBracket() *Token {
	resyncPoint := t.currentIndex
	resyncIfMemberAccess := false
	for {
		t.tokenStart = t.currentIndex
		c := t.getChar()
		switch c {
		case ' ', '\t', '\f', '\v', charNoBreakSpace, charNextLine:
			resyncIfMemberAccess = true
			t.skipWhiteSpace()
			continue
		case '#':
			resyncIfMemberAccess = true
			t.scanLineComment()
			continue
		case '<':
			if t.peekChar() == '#' {
				resyncIfMemberAccess = false
				t.skipChar()
				t.scanBlockComment()
				continue
			}
			t.ungetChar()
		case '[':
			return t.newToken(LBracket)
		case '.', ':':
			if resyncIfMemberAccess {
				// C# passes a script index where Resync expects an input offset; outside a nested scan the two
				// agree, and this port keeps the call as written.
				t.resync(resyncPoint)
			} else {
				t.ungetChar()
			}
		default:
			if isWhitespace(c) {
				resyncIfMemberAccess = true
				t.skipWhiteSpace()
				continue
			}
			t.ungetChar()
		}
		return nil
	}
}

// scanAssemblyNameSpecToken is ScanAssemblyNameSpecToken: whitespace, ',', '=', ']' and newlines end a token.
func (t *tokenizer) scanAssemblyNameSpecToken(sb *[]uint16) {
	t.skipWhiteSpace()
	t.tokenStart = t.currentIndex
	for {
		c := t.getChar()
		if forceStartNewTokenInAssemblyNameSpec(c) {
			t.ungetChar()
			break
		}
		*sb = append(*sb, c)
	}
	tok := t.newToken(Identifier)
	tok.Flags |= FlagTypeName
	t.skipWhiteSpace()
}

// getAssemblyNameSpec is GetAssemblyNameSpec: the assembly name and its properties, validated by nobody here, as
// C# defers validation to the CLR.
func (t *tokenizer) getAssemblyNameSpec() string {
	var sb []uint16
	t.scanAssemblyNameSpecToken(&sb)
	for t.peekChar() == ',' {
		t.tokenStart = t.currentIndex
		sb = append(sb, ',', ' ')
		t.skipChar()
		t.newToken(Comma)
		t.scanAssemblyNameSpecToken(&sb)
		if t.peekChar() == '=' {
			t.tokenStart = t.currentIndex
			sb = append(sb, '=')
			t.skipChar()
			t.newToken(Equals)
			t.scanAssemblyNameSpecToken(&sb)
		}
	}
	return utf16ToString(sb)
}

// keywordTable is s_keywordTable, keyed by OrdinalUpper as the C# dictionary is OrdinalIgnoreCase.
var keywordTable = func() map[string]TokenKind {
	text := []string{
		"elseif", "if", "else", "switch", "foreach", "from", "in", "for", "while", "until", "do", "try", "catch",
		"finally", "trap", "data", "return", "continue", "break", "exit", "throw", "begin", "process", "end",
		"dynamicparam", "function", "filter", "param", "class", "define", "var", "using", "workflow", "parallel",
		"sequence", "inlinescript", "configuration", "public", "private", "static", "interface", "enum",
		"namespace", "module", "type", "assembly", "command", "hidden", "base", "default", "clean",
	}
	kinds := []TokenKind{
		ElseIf, If, Else, Switch, Foreach, From, In, For, While, Until, Do, Try, Catch, Finally, Trap, Data, Return,
		Continue, Break, Exit, Throw, Begin, Process, End, Dynamicparam, Function, Filter, Param, Class, Define, Var,
		Using, Workflow, Parallel, Sequence, InlineScript, Configuration, Public, Private, Static, Interface, Enum,
		Namespace, Module, Type, Assembly, Command, Hidden, Base, Default, Clean,
	}
	m := make(map[string]TokenKind, len(text))
	for i, s := range text {
		m[OrdinalUpper(s)] = kinds[i]
	}
	return m
}()

// operatorTable is s_operatorTable, keyed by OrdinalUpper.
var operatorTable = func() map[string]TokenKind {
	text := []string{
		"bnot", "not", "eq", "ieq", "ceq", "ne", "ine", "cne", "ge", "ige", "cge", "gt", "igt", "cgt", "lt", "ilt",
		"clt", "le", "ile", "cle", "like", "ilike", "clike", "notlike", "inotlike", "cnotlike", "match", "imatch",
		"cmatch", "notmatch", "inotmatch", "cnotmatch", "replace", "ireplace", "creplace", "contains", "icontains",
		"ccontains", "notcontains", "inotcontains", "cnotcontains", "in", "iin", "cin", "notin", "inotin",
		"cnotin", "split", "isplit", "csplit", "isnot", "is", "as", "f", "and", "band", "or", "bor", "xor", "bxor",
		"join", "shl", "shr",
	}
	kinds := []TokenKind{
		Bnot, Not, Ieq, Ieq, Ceq, Ine, Ine, Cne, Ige, Ige, Cge, Igt, Igt, Cgt, Ilt, Ilt, Clt, Ile, Ile, Cle, Ilike,
		Ilike, Clike, Inotlike, Inotlike, Cnotlike, Imatch, Imatch, Cmatch, Inotmatch, Inotmatch, Cnotmatch,
		Ireplace, Ireplace, Creplace, Icontains, Icontains, Ccontains, Inotcontains, Inotcontains, Cnotcontains,
		Iin, Iin, Cin, Inotin, Inotin, Cnotin, Isplit, Isplit, Csplit, IsNot, Is, As, Format, And, Band, Or, Bor,
		Xor, Bxor, Join, Shl, Shr,
	}
	m := make(map[string]TokenKind, len(text))
	for i, s := range text {
		m[OrdinalUpper(s)] = kinds[i]
	}
	return m
}()

// isKeyword is IsKeyword. No dynamic keywords are registered in a fresh ParseInput session (contract decision 5).
func isKeyword(s string) bool {
	_, ok := keywordTable[OrdinalUpper(s)]
	return ok
}

// nextToken is NextToken.
func (t *tokenizer) nextToken() *Token {
	for {
		t.tokenStart = t.currentIndex
		c := t.getChar()
		switch c {
		case ' ', '\t', '\f', '\v', charNoBreakSpace, charNextLine:
			t.skipWhiteSpace()
			continue

		case '\'', charQuoteSingleLeft, charQuoteSingleRight, charQuoteSingleBase, charQuoteReversed:
			return t.scanStringLiteralToken()

		case '"', charQuoteDoubleLeft, charQuoteDoubleRight, charQuoteLowDoubleLft:
			return t.scanStringExpandableToken()

		case '@':
			c1 := t.getChar()
			if c1 == '{' {
				return t.newToken(AtCurly)
			}
			if c1 == '(' {
				return t.newToken(AtParen)
			}
			if isSingleQuote(c1) {
				return t.scanHereStringLiteral()
			}
			if isDoubleQuote(c1) {
				return t.scanHereStringExpandable()
			}
			t.ungetChar()
			if isVariableStart(c1) {
				return t.scanVariable(true, false)
			}
			t.reportErrorAt(t.currentIndex-1, "UnrecognizedToken")
			return t.newToken(Unknown)

		case '#':
			t.scanLineComment()
			continue

		case '\n':
			return t.newToken(NewLine)

		case '\r':
			t.normalizeCRLF(c)
			return t.newToken(NewLine)

		case '`':
			c1 := t.getChar()
			if c1 == '\r' {
				t.normalizeCRLF(c1)
			}
			if c1 == '\n' || c1 == '\r' {
				t.newToken(LineContinuation)
				continue
			}
			if netIsWhiteSpace(c1) {
				t.skipWhiteSpace()
				continue
			}
			if c1 == 0 && t.atEOF() {
				t.reportIncompleteInputAt(t.currentIndex, "IncompleteString")
			}
			ch, surrogate := t.backtick(c1)
			return t.scanGenericTokenChars(ch, surrogate)

		case '=':
			return t.checkOperatorInCommandMode(c, Equals)

		case '+':
			c1 := t.peekChar()
			if c1 == '+' {
				t.skipChar()
				return t.checkOperatorInCommandMode2(c, c1, PlusPlus)
			}
			if c1 == '=' {
				t.skipChar()
				return t.checkOperatorInCommandMode2(c, c1, PlusEquals)
			}
			if t.AllowSignedNumbers && (netIsDigit(c1) || c1 == '.') {
				return t.scanNumber(c)
			}
			return t.checkOperatorInCommandMode(c, Plus)

		case '-', charEmDash, charEnDash, charHorizontalBar:
			c1 := t.peekChar()
			if isDash(c1) {
				t.skipChar()
				return t.checkOperatorInCommandMode2(c, c1, MinusMinus)
			}
			if c1 == '=' {
				t.skipChar()
				return t.checkOperatorInCommandMode2(c, c1, MinusEquals)
			}
			if netIsLetter(c1) || c1 == '_' || c1 == '?' {
				return t.scanParameter()
			}
			if t.AllowSignedNumbers && (netIsDigit(c1) || c1 == '.') {
				return t.scanNumber(c)
			}
			return t.checkOperatorInCommandMode(c, Minus)

		case '*':
			c1 := t.peekChar()
			if c1 == '=' {
				t.skipChar()
				return t.checkOperatorInCommandMode2(c, c1, MultiplyEquals)
			}
			if c1 == '>' {
				t.skipChar()
				c1 = t.peekChar()
				if c1 == '>' {
					t.skipChar()
					return t.newFileRedirectionToken(0, true, false)
				}
				if c1 == '&' {
					t.skipChar()
					if t.peekChar() == '1' {
						t.skipChar()
						return t.newMergingRedirectionToken(0, 1)
					}
					t.ungetChar()
				}
				return t.newFileRedirectionToken(0, false, false)
			}
			return t.checkOperatorInCommandMode(c, Multiply)

		case '/':
			if c1 := t.peekChar(); c1 == '=' {
				t.skipChar()
				return t.checkOperatorInCommandMode2(c, c1, DivideEquals)
			}
			return t.checkOperatorInCommandMode(c, Divide)

		case '%':
			if c1 := t.peekChar(); c1 == '=' {
				t.skipChar()
				return t.checkOperatorInCommandMode2(c, c1, RemainderEquals)
			}
			return t.checkOperatorInCommandMode(c, Rem)

		case '$':
			if t.peekChar() == '(' {
				t.skipChar()
				return t.newToken(DollarParen)
			}
			return t.scanVariable(false, false)

		case '<':
			if t.peekChar() == '#' {
				t.skipChar()
				t.scanBlockComment()
				continue
			}
			return t.newInputRedirectionToken()

		case '>':
			if t.peekChar() == '>' {
				t.skipChar()
				return t.newFileRedirectionToken(1, true, false)
			}
			return t.newFileRedirectionToken(1, false, false)

		case '(':
			return t.newToken(LParen)
		case ')':
			return t.newToken(RParen)
		case '[':
			if t.inCommandMode() && !forceStartNewToken(t.peekChar()) {
				return t.scanGenericTokenChar('[')
			}
			return t.newToken(LBracket)
		case ']':
			return t.newToken(RBracket)
		case '{':
			return t.newToken(LCurly)
		case '}':
			return t.newToken(RCurly)
		case '.':
			return t.scanDot()
		case ';':
			return t.newToken(Semi)
		case ',':
			return t.newToken(Comma)

		case '0', '7', '8', '9':
			return t.scanNumber(c)

		case '1', '2', '3', '4', '5', '6':
			if t.peekChar() == '>' {
				t.skipChar()
				c1 := t.peekChar()
				if c1 == '>' {
					t.skipChar()
					return t.newFileRedirectionToken(int(c-'0'), true, true)
				}
				if c1 == '&' {
					t.skipChar()
					c1 = t.peekChar()
					if c1 == '1' || c1 == '2' {
						t.skipChar()
						return t.newMergingRedirectionToken(int(c-'0'), int(c1-'0'))
					}
					t.ungetChar()
				}
				return t.newFileRedirectionToken(int(c-'0'), false, true)
			}
			return t.scanNumber(c)

		case '&':
			if t.peekChar() == '&' {
				t.skipChar()
				return t.newToken(AndAnd)
			}
			return t.newToken(Ampersand)

		case '|':
			if t.peekChar() == '|' {
				t.skipChar()
				return t.newToken(OrOr)
			}
			return t.newToken(Pipe)

		case '!':
			c1 := t.peekChar()
			if (t.inCommandMode() && !forceStartNewToken(c1)) || (t.inExpressionMode() && isIdentifierStart(c1)) {
				return t.scanGenericTokenChar(c)
			}
			if t.inExpressionMode() && (netIsDigit(c1) || c1 == '.') {
				// Check whether a number follows, then rescan from the '!'.
				_, _, _, _, ok := t.scanNumberHelper(c)
				t.currentIndex = t.tokenStart
				c = t.getChar()
				if !ok {
					return t.scanGenericTokenChar(c)
				}
			}
			return t.newToken(Exclaim)

		case ':':
			if t.peekChar() == ':' {
				t.skipChar()
				if t.inCommandMode() && !t.WantSimpleName && !forceStartNewToken(t.peekChar()) {
					return t.scanGenericToken([]uint16{':', ':'})
				}
				return t.newToken(ColonColon)
			}
			if t.inCommandMode() {
				return t.scanLabel()
			}
			return t.newToken(Colon)

		case 0:
			if t.atEOF() {
				return t.saveToken(t.makeToken(t.newScriptExtent(t.tokenStart+1, t.tokenStart+1), EndOfInput,
					ClassToken, FlagNone))
			}
			return t.scanGenericTokenChar(c)
		}

		if c == '?' && t.inExpressionMode() {
			c1 := t.peekChar()
			if c1 == '?' {
				t.skipChar()
				if t.peekChar() == '=' {
					t.skipChar()
					return t.newToken(QuestionQuestionEquals)
				}
				return t.newToken(QuestionQuestion)
			}
			return t.newToken(QuestionMark)
		}
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
			return t.scanIdentifier(c)
		}
		if isWhitespace(c) {
			t.skipWhiteSpace()
			continue
		}
		if netIsLetter(c) {
			return t.scanIdentifier(c)
		}
		return t.scanGenericTokenChar(c)
	}
}

// scanTokens drives only the tokenizer over input in one mode, collecting tokens to EndOfInput, for tests and
// the oracle's mode-specific token checks. The parser normally switches modes as it goes, so the stream here
// matches ParseInput's only for input that stays in that mode.
func scanTokens(input string, mode TokenizerMode) (tokens []*Token, err error) {
	src := newSource(input)
	p := &parser{src: src, lim: DefaultLimits}
	p.tok = newTokenizer(p)
	p.tok.initialize(src, &tokens)
	p.tok.Mode = mode
	defer func() {
		if r := recover(); r != nil {
			a, ok := r.(abortParse)
			if !ok {
				panic(r)
			}
			tokens, err = nil, a.err
		}
	}()
	for {
		if p.tok.nextToken().Kind == EndOfInput {
			return tokens, nil
		}
	}
}
