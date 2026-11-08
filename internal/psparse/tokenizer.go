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

// tokenizer is PowerShell's Tokenizer. The parser drives it: it switches Mode, rewinds with resync and rescans,
// so this API follows tokenizer.cs member for member. Each method's doc names the C# member it ports.
type tokenizer struct {
	parser *parser

	Mode                           TokenizerMode
	AllowSignedNumbers             bool
	ForceEndNumberOnTernaryOpChars bool
	WantSimpleName                 bool
	InWorkflowContext              bool

	// tokenList collects every token when non-nil (TokenList); the parser sets it to nil around rescans it does
	// not want recorded.
	tokenList  *[]*Token
	FirstToken *Token
	LastToken  *Token

	src source
	// The remaining state is the tokenizer owner's to define while porting.
}

// tokenizerState is TokenizerState, the snapshot a nested scan restores.
type tokenizerState struct {
	// Fields are the tokenizer owner's to define.
}

func newTokenizer(p *parser) *tokenizer { return &tokenizer{parser: p} }

// initialize is Initialize.
func (t *tokenizer) initialize(src source, tokenList *[]*Token) {
	t.src = src
	t.tokenList = tokenList
	t.Mode = ModeCommand
}

// nextToken is NextToken.
func (t *tokenizer) nextToken() *Token { panic(notPorted) }

// skipNewlines is SkipNewlines.
func (t *tokenizer) skipNewlines(skipSemis bool) { panic(notPorted) }

// getRestorePoint is GetRestorePoint.
func (t *tokenizer) getRestorePoint() int { panic(notPorted) }

// resync is Resync(int).
func (t *tokenizer) resync(start int) { panic(notPorted) }

// resyncToken is Resync(Token).
func (t *tokenizer) resyncToken(tok *Token) { t.resync(tok.Extent.Start) }

// startNestedScan is StartNestedScan, for the text of an unscanned subexpression in an expandable string.
func (t *tokenizer) startNestedScan(nested *Token) tokenizerState { panic(notPorted) }

// finishNestedScan is FinishNestedScan.
func (t *tokenizer) finishNestedScan(st tokenizerState) { panic(notPorted) }

// replaceSavedTokens is ReplaceSavedTokens.
func (t *tokenizer) replaceSavedTokens(first, last, replacement *Token) { panic(notPorted) }

// currentExtent is CurrentExtent.
func (t *tokenizer) currentExtent() Extent { panic(notPorted) }

// isAtEndOfScript is IsAtEndOfScript.
func (t *tokenizer) isAtEndOfScript(e Extent, checkCommentsAndWhitespace bool) bool { panic(notPorted) }

// isPipeContinuation is IsPipeContinuation.
func (t *tokenizer) isPipeContinuation(e Extent) bool { panic(notPorted) }

// getScriptRequirements is GetScriptRequirements.
func (t *tokenizer) getScriptRequirements() *ScriptRequirements { panic(notPorted) }

// getVerbatimCommandArgument is GetVerbatimCommandArgument, the text after --%.
func (t *tokenizer) getVerbatimCommandArgument() *Token { panic(notPorted) }

// getMemberAccessOperator is GetMemberAccessOperator.
func (t *tokenizer) getMemberAccessOperator(allowLBracket bool) *Token { panic(notPorted) }

// getInvokeMemberOpenParen is GetInvokeMemberOpenParen.
func (t *tokenizer) getInvokeMemberOpenParen() *Token { panic(notPorted) }

// getLBracket is GetLBracket.
func (t *tokenizer) getLBracket() *Token { panic(notPorted) }

// getAssemblyNameSpec is GetAssemblyNameSpec.
func (t *tokenizer) getAssemblyNameSpec() string { panic(notPorted) }

// checkAstIsBeforeSignature is CheckAstIsBeforeSignature.
func (t *tokenizer) checkAstIsBeforeSignature(n Ast) { panic(notPorted) }
