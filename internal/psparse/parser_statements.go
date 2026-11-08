// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// The statement half of Parser.cs: script blocks and named blocks, statements, pipelines, commands and their
// arguments, redirections. Each method's doc names the C# rule it ports; an out or ref parameter there becomes a
// result or a pointer here. Error recovery branches are left out, since reportError stops the parse.

// commandArgumentContext is CommandArgumentContext.
type commandArgumentContext int

const (
	argCommandName                        commandArgumentContext = 0x01
	argCommandNameAfterInvocationOperator commandArgumentContext = 0x02 | argCommandName
	argFileName                           commandArgumentContext = 0x04
	argCommandArgument                    commandArgumentContext = 0x08
	argSwitchCondition                    commandArgumentContext = 0x10
)

// scriptBlockRule is ScriptBlockRule; lCurly is nil for a whole script.
func (p *parser) scriptBlockRule(lCurly *Token, isFilter bool, predefined StatementAst) *ScriptBlockAst {
	panic(notPorted)
}

// completeScriptBlockBody is CompleteScriptBlockBody.
func (p *parser) completeScriptBlockBody(lCurly *Token, bodyExtent *Extent) (fullBodyExtent Extent, ok bool) {
	panic(notPorted)
}

// scriptBlockBodyRule is ScriptBlockBodyRule.
func (p *parser) scriptBlockBodyRule(lCurly *Token, usingStatements []*UsingStatementAst, paramBlock *ParamBlockAst, isFilter bool, predefined StatementAst) *ScriptBlockAst {
	panic(notPorted)
}

// namedBlockListRule is NamedBlockListRule.
func (p *parser) namedBlockListRule(lCurly *Token, usingStatements []*UsingStatementAst, paramBlock *ParamBlockAst) *ScriptBlockAst {
	panic(notPorted)
}

// statementBlockRule is StatementBlockRule.
func (p *parser) statementBlockRule() *StatementBlockAst { panic(notPorted) }

// statementListRule is StatementListRule; it returns the extent the statements cover.
func (p *parser) statementListRule(statements *[]StatementAst, traps *[]*TrapStatementAst) Extent {
	panic(notPorted)
}

// statementRule is StatementRule. DSC configuration and dynamic keywords report ErrDynamicKeywords.
func (p *parser) statementRule() StatementAst { panic(notPorted) }

// simpleNameRule is SimpleNameRule(out Token).
func (p *parser) simpleNameRule() (*StringConstantExpressionAst, *Token) { panic(notPorted) }

// labelOrKeyRule is LabelOrKeyRule.
func (p *parser) labelOrKeyRule() ExpressionAst { panic(notPorted) }

// breakStatementRule is BreakStatementRule.
func (p *parser) breakStatementRule(breakToken *Token) *BreakStatementAst { panic(notPorted) }

// continueStatementRule is ContinueStatementRule.
func (p *parser) continueStatementRule(continueToken *Token) *ContinueStatementAst { panic(notPorted) }

// returnStatementRule is ReturnStatementRule.
func (p *parser) returnStatementRule(t *Token) *ReturnStatementAst { panic(notPorted) }

// exitStatementRule is ExitStatementRule.
func (p *parser) exitStatementRule(t *Token) *ExitStatementAst { panic(notPorted) }

// throwStatementRule is ThrowStatementRule.
func (p *parser) throwStatementRule(t *Token) *ThrowStatementAst { panic(notPorted) }

// labeledStatementRule is LabeledStatementRule.
func (p *parser) labeledStatementRule(label *Token) StatementAst { panic(notPorted) }

// blockStatementRule is BlockStatementRule (workflow parallel and sequence, which pwsh 7 rejects).
func (p *parser) blockStatementRule(kindToken *Token) StatementAst { panic(notPorted) }

// inlineScriptRule is InlineScriptRule.
func (p *parser) inlineScriptRule(inlineScriptToken *Token, elements *[]CommandElementAst) bool {
	panic(notPorted)
}

// ifStatementRule is IfStatementRule.
func (p *parser) ifStatementRule(ifToken *Token) StatementAst { panic(notPorted) }

// switchStatementRule is SwitchStatementRule; labelToken may be nil.
func (p *parser) switchStatementRule(labelToken, switchToken *Token) StatementAst { panic(notPorted) }

// getWordOrExpression is GetWordOrExpression.
func (p *parser) getWordOrExpression(keywordToken *Token) ExpressionAst { panic(notPorted) }

// foreachStatementRule is ForeachStatementRule.
func (p *parser) foreachStatementRule(labelToken, forEachToken *Token) StatementAst { panic(notPorted) }

// forStatementRule is ForStatementRule.
func (p *parser) forStatementRule(labelToken, forToken *Token) StatementAst { panic(notPorted) }

// whileStatementRule is WhileStatementRule.
func (p *parser) whileStatementRule(labelToken, whileToken *Token) StatementAst { panic(notPorted) }

// doWhileStatementRule is DoWhileStatementRule (do-while and do-until).
func (p *parser) doWhileStatementRule(labelToken, doToken *Token) StatementAst { panic(notPorted) }

// functionDeclarationRule is FunctionDeclarationRule.
func (p *parser) functionDeclarationRule(functionToken *Token) StatementAst { panic(notPorted) }

// functionParameterDeclarationRule is FunctionParameterDeclarationRule.
func (p *parser) functionParameterDeclarationRule() (params []*ParameterAst, endErrorStatement Extent, rParen *Token) {
	panic(notPorted)
}

// trapStatementRule is TrapStatementRule.
func (p *parser) trapStatementRule(trapToken *Token) StatementAst { panic(notPorted) }

// catchBlockRule is CatchBlockRule; nil when no catch follows.
func (p *parser) catchBlockRule(endErrorStatement *Extent) *CatchClauseAst { panic(notPorted) }

// tryStatementRule is TryStatementRule.
func (p *parser) tryStatementRule(tryToken *Token) StatementAst { panic(notPorted) }

// dataStatementRule is DataStatementRule.
func (p *parser) dataStatementRule(dataToken *Token) StatementAst { panic(notPorted) }

// pipelineChainRule is PipelineChainRule.
func (p *parser) pipelineChainRule() PipelineBaseAst { panic(notPorted) }

// pipelineRule is PipelineRule.
func (p *parser) pipelineRule(startExpression ExpressionAst, allowBackground bool) PipelineBaseAst {
	panic(notPorted)
}

// redirectionRule is RedirectionRule; redirections holds the redirections seen so far, indexed by stream.
func (p *parser) redirectionRule(redirectionToken *Token, redirections *[7]RedirectionAst, extent *Extent) RedirectionAst {
	panic(notPorted)
}

// getSingleCommandArgument is GetSingleCommandArgument.
func (p *parser) getSingleCommandArgument(context commandArgumentContext) ExpressionAst {
	panic(notPorted)
}

// getCommandArgument is GetCommandArgument.
func (p *parser) getCommandArgument(context commandArgumentContext, t *Token) ExpressionAst {
	panic(notPorted)
}

// commandRule is CommandRule; it returns a *CommandAst or *CommandExpressionAst, or nil when no command starts.
func (p *parser) commandRule(forDynamicKeyword bool) Ast { panic(notPorted) }
