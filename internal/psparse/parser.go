// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import "unicode"

// Result is a successful parse: the tree and every token, as Parser.ParseInput returns them.
type Result struct {
	Ast    *ScriptBlockAst
	Tokens []*Token
	src    source
}

// Text returns the input an extent covers, as written.
func (r *Result) Text(e Extent) string { return r.src.slice(e) }

// Parse parses input as PowerShell's Parser.ParseInput does. It returns the first parse error, or one of the
// Lemongrass errors for input past lim, and never a partial tree.
//
// ParseInput also runs PowerShell's post-parse semantic checks; this port does not, so input those checks reject
// parses here. That direction is safe for the gate: PowerShell runs nothing from such input, and the tree that is
// judged is the one PowerShell would have refused.
func Parse(input string, lim Limits) (res *Result, err error) {
	src := newSource(input)
	if lim.MaxInput > 0 && len(src.text) > lim.MaxInput {
		return nil, newParseError(Extent{}, ErrInputTooLarge, false)
	}
	p := &parser{src: src, lim: lim}
	p.tok = newTokenizer(p)
	var tokens []*Token
	p.tok.initialize(src, &tokens)
	defer func() {
		if r := recover(); r != nil {
			a, ok := r.(abortParse)
			if !ok {
				panic(r)
			}
			res, err = nil, a.err
		}
	}()
	ast := p.scriptBlockRule(nil, false, nil)
	ast.ScriptRequirements = p.tok.getScriptRequirements()
	return &Result{Ast: ast, Tokens: tokens, src: src}, nil
}

// parser is PowerShell's Parser. Its rule methods live in parser_statements.go and parser_expressions.go under
// their two owners; this file holds the shared token plumbing, extents, errors and the depth bound.
type parser struct {
	tok        *tokenizer
	ungotToken *Token
	src        source
	lim        Limits
	depth      int

	disableCommaOperator bool
	inConfiguration      bool
}

// enter counts one level of nesting and refuses input past lim.MaxDepth; every recursive rule calls it with a
// deferred leave.
func (p *parser) enter() {
	p.depth++
	if p.lim.MaxDepth > 0 && p.depth > p.lim.MaxDepth {
		panic(abortParse{newParseError(p.tok.currentExtent(), ErrNestingTooDeep, false)})
	}
}

func (p *parser) leave() { p.depth-- }

// reportError is ReportError: it stops the parse at the first error.
func (p *parser) reportError(ext Extent, id string, args ...any) {
	panic(abortParse{newParseError(ext, id, false, args...)})
}

// reportIncompleteInput is ReportIncompleteInput: the error is marked incomplete input only when ext is at the
// end of the script, comments and whitespace aside.
func (p *parser) reportIncompleteInput(ext Extent, id string, args ...any) {
	p.reportIncompleteInputAt(ext, ext, id, args...)
}

// reportIncompleteInputAt is the two-extent ReportIncompleteInput: the error sits at pos, and whether it is
// incomplete input is judged at detected.
func (p *parser) reportIncompleteInputAt(pos, detected Extent, id string, args ...any) {
	incomplete := p.tok.isAtEndOfScript(detected, true)
	panic(abortParse{newParseError(pos, id, incomplete, args...)})
}

// skipNewlines is SkipNewlines.
func (p *parser) skipNewlines() {
	if p.ungotToken == nil || p.ungotToken.Kind == NewLine {
		p.ungotToken = nil
		p.tok.skipNewlines(false)
	}
}

// skipNewlinesAndSemicolons is SkipNewlinesAndSemicolons.
func (p *parser) skipNewlinesAndSemicolons() {
	if p.ungotToken == nil || p.ungotToken.Kind == NewLine || p.ungotToken.Kind == Semi {
		p.ungotToken = nil
		p.tok.skipNewlines(true)
	}
}

// nextToken is NextToken.
func (p *parser) nextToken() *Token {
	t := p.ungotToken
	if t == nil {
		t = p.tok.nextToken()
	}
	p.ungotToken = nil
	return t
}

// peekToken is PeekToken.
func (p *parser) peekToken() *Token {
	t := p.ungotToken
	if t == nil {
		t = p.tok.nextToken()
	}
	if p.ungotToken == nil {
		p.ungotToken = t
	}
	return t
}

// nextMemberAccessToken is NextMemberAccessToken; nil when a token is already pushed back.
func (p *parser) nextMemberAccessToken(allowLBracket bool) *Token {
	if p.ungotToken != nil {
		return nil
	}
	return p.tok.getMemberAccessOperator(allowLBracket)
}

// nextInvokeMemberToken is NextInvokeMemberToken.
func (p *parser) nextInvokeMemberToken() *Token {
	if p.ungotToken != nil {
		return nil
	}
	return p.tok.getInvokeMemberOpenParen()
}

// nextLBracket is NextLBracket.
func (p *parser) nextLBracket() *Token {
	if p.ungotToken != nil {
		if p.ungotToken.Kind == LBracket {
			return p.nextToken()
		}
		return nil
	}
	return p.tok.getLBracket()
}

// getVerbatimCommandArgumentToken is GetVerbatimCommandArgumentToken.
func (p *parser) getVerbatimCommandArgumentToken() *Token {
	if p.ungotToken == nil || p.ungotToken.Kind == Parameter {
		p.ungotToken = nil
		return p.tok.getVerbatimCommandArgument()
	}
	return nil
}

// skipToken is SkipToken: it consumes the token peekToken pushed back.
func (p *parser) skipToken() { p.ungotToken = nil }

// ungetToken is UngetToken; PowerShell supports one token of lookahead.
func (p *parser) ungetToken(t *Token) { p.ungotToken = t }

// setTokenizerMode is SetTokenizerMode: a pushed-back token whose kind depends on the mode is rescanned.
func (p *parser) setTokenizerMode(mode TokenizerMode) {
	if mode != p.tok.Mode && p.ungotToken != nil && !p.ungotToken.Kind.HasTrait(FlagParseModeInvariant) {
		p.resyncToken(p.ungotToken)
	}
	p.tok.Mode = mode
}

// resyncToken is Resync(Token).
func (p *parser) resyncToken(t *Token) {
	p.ungotToken = nil
	p.tok.resyncToken(t)
}

// resync is Resync(int).
func (p *parser) resync(restorePoint int) {
	p.ungotToken = nil
	p.tok.resync(restorePoint)
}

// isSpecificParameter is IsSpecificParameter: the token's name is a prefix of parameter, compared the way .NET's
// OrdinalIgnoreCase does, by upper-casing each character.
func isSpecificParameter(t *Token, parameter string) bool {
	name, full := []rune(t.ParameterName), []rune(parameter)
	if len(name) > len(full) {
		return false
	}
	for i, r := range name {
		if unicode.ToUpper(r) != unicode.ToUpper(full[i]) {
			return false
		}
	}
	return true
}

// requireStatementTerminator is RequireStatementTerminator.
func (p *parser) requireStatementTerminator() {
	t := p.peekToken()
	switch {
	case t.Kind == NewLine || t.Kind == Semi:
		p.skipToken()
	case t.Kind != EndOfInput:
		p.reportIncompleteInput(t.Extent, "MissingStatementTerminator")
	}
}

// emptyExtent is PowerShell's EmptyScriptExtent, which is distinct from an empty range at offset 0.
var emptyExtent = Extent{Start: -1, End: -1}

// extentOf is ExtentOf: from the start of first to the end of last. emptyExtent yields the other.
func extentOf(first, last Extent) Extent {
	if first == emptyExtent {
		return last
	}
	if last == emptyExtent {
		return first
	}
	return Extent{Start: first.Start, End: last.End}
}

// before is Before: the empty extent just before e.
func before(e Extent) Extent {
	off := e.Start - 1
	if off < 0 {
		off = 0
	}
	return Extent{Start: off, End: off}
}

// after is After: the empty extent at the end of e.
func after(e Extent) Extent { return Extent{Start: e.End, End: e.End} }

// lastCharacterOf is LastCharacterOf.
func lastCharacterOf(e Extent) Extent {
	off := e.End - 1
	if off < 0 {
		off = 0
	}
	return Extent{Start: off, End: off}
}

// text returns the source text of e.
func (p *parser) text(e Extent) string { return p.src.slice(e) }
