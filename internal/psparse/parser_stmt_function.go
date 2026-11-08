// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// Function declarations, trap, try and catch, data sections, and the workflow parallel, sequence and inlinescript
// forms, which pwsh 7 still parses. Each method's doc names the C# rule it ports. As in parser_statements.go,
// recovery after an error is left out.

// blockStatementRule is BlockStatementRule (workflow parallel and sequence).
func (p *parser) blockStatementRule(kindToken *Token) StatementAst {
	body := p.statementBlockRule()
	if body == nil {
		p.reportIncompleteInput(after(kindToken.Extent), "MissingStatementAfterKeyword", kindToken.Text)
	}
	return newBlockStatementAst(extentOf(kindToken.Extent, body.Extent()), kindToken, body)
}

// inlineScriptRule is InlineScriptRule: the workflow inlinescript command and its script block. C# returns false
// only after reporting an error, which stops the parse here.
func (p *parser) inlineScriptRule(inlineScriptToken *Token, elements *[]CommandElementAst) bool {
	inlineScriptToken.Flags |= FlagCommandName
	*elements = append(*elements, newStringConstantExpressionAst(inlineScriptToken.Extent, inlineScriptToken.Text, BareWord))

	p.skipNewlines()
	lCurly := p.nextToken()
	if lCurly.Kind != LCurly {
		p.ungetToken(lCurly)
		p.reportIncompleteInput(after(inlineScriptToken.Extent), "MissingStatementAfterKeyword", inlineScriptToken.Text)
	}
	*elements = append(*elements, p.scriptBlockExpressionRule(lCurly))
	return true
}

// functionDeclarationRule is FunctionDeclarationRule: function, filter and workflow.
func (p *parser) functionDeclarationRule(functionToken *Token) StatementAst {
	p.skipNewlines()
	functionNameToken := p.nextToken()
	switch functionNameToken.Kind {
	case Pipe, LParen, LCurly, AtParen, AtCurly, RCurly, RParen, EndOfInput, Semi, Redirection, RedirectInStd, AndAnd, OrOr,
		Ampersand, Variable, SplattedVariable, HereStringExpandable, HereStringLiteral, StringLiteral, StringExpandable:
		p.ungetToken(functionNameToken)
		p.reportIncompleteInput(after(functionToken.Extent), "MissingNameAfterKeyword", functionToken.Text)
	}

	// A keyword is a valid function name.
	functionNameToken.Flags &^= FlagKeyword

	parameters, _, rParen := p.functionParameterDeclarationRule()

	lCurly := p.nextToken()
	if lCurly.Kind != LCurly {
		p.ungetToken(lCurly)
		endErrorStatement := functionNameToken.Extent
		if rParen != nil {
			endErrorStatement = rParen.Extent
		}
		p.reportIncompleteInput(after(endErrorStatement), "MissingFunctionBody")
	}

	isFilter := functionToken.Kind == Filter
	isWorkflow := functionToken.Kind == Workflow

	oldWorkflowContext := p.tok.InWorkflowContext
	p.tok.InWorkflowContext = isWorkflow
	scriptBlock := p.scriptBlockRule(lCurly, isFilter, nil)
	p.tok.InWorkflowContext = oldWorkflowContext

	return newFunctionDefinitionAst(extentOf(functionToken.Extent, scriptBlock.Extent()), isFilter, isWorkflow, functionNameToken, parameters, scriptBlock)
}

// functionParameterDeclarationRule is FunctionParameterDeclarationRule: the parenthesized parameters after a
// function name, nil when there are none. endErrorStatement is always emptyExtent, since C# sets it only next to
// an error report; rParen is nil without parentheses.
func (p *parser) functionParameterDeclarationRule() (params []*ParameterAst, endErrorStatement Extent, rParen *Token) {
	p.skipNewlines()
	lParen := p.peekToken()
	if lParen.Kind == LParen {
		p.skipToken()
		params = p.parameterListRule()
		p.skipNewlines()

		rParen = p.nextToken()
		if rParen.Kind != RParen {
			p.ungetToken(rParen)
			end := lParen.Extent
			if len(params) > 0 {
				end = params[len(params)-1].Extent()
			}
			p.reportIncompleteInput(after(end), "MissingEndParenthesisInFunctionParameterList")
		}
		p.skipNewlines()
	}
	return params, emptyExtent, rParen
}

// trapStatementRule is TrapStatementRule.
func (p *parser) trapStatementRule(trapToken *Token) StatementAst {
	restorePoint := p.tok.getRestorePoint()
	p.skipNewlines()
	typ := p.attributeRule()
	typeConstraintAst, _ := typ.(*TypeConstraintAst)
	if typ != nil && typeConstraintAst == nil {
		// An attribute is no trap type; it starts the body's first statement instead.
		p.resync(restorePoint)
	}

	body := p.statementBlockRule()
	if body == nil {
		endErrorStatement := trapToken.Extent
		if typeConstraintAst != nil {
			endErrorStatement = typeConstraintAst.Extent()
		}
		p.reportIncompleteInput(after(endErrorStatement), "MissingTrapStatement")
	}
	return newTrapStatementAst(extentOf(trapToken.Extent, body.Extent()), typeConstraintAst, body)
}

// catchBlockRule is CatchBlockRule; nil when no catch follows. C#'s ref parameters only carry recovery state.
func (p *parser) catchBlockRule() *CatchClauseAst {
	p.skipNewlines()
	catchToken := p.nextToken()
	if catchToken.Kind != Catch {
		p.ungetToken(catchToken)
		return nil
	}

	var exceptionTypes []*TypeConstraintAst
	var commaToken *Token
	for {
		restorePoint := p.tok.getRestorePoint()
		p.skipNewlines()
		typ := p.attributeRule()
		if typ == nil {
			if commaToken != nil {
				p.reportIncompleteInput(after(commaToken.Extent), "MissingTypeLiteralToken")
			}
			break
		}
		typeConstraintAst, ok := typ.(*TypeConstraintAst)
		if !ok {
			p.resync(restorePoint)
			break
		}
		exceptionTypes = append(exceptionTypes, typeConstraintAst)

		p.skipNewlines()
		commaToken = p.peekToken()
		if commaToken.Kind != Comma {
			break
		}
		p.skipToken()
	}

	handler := p.statementBlockRule()
	if handler == nil {
		endErrorStatement := catchToken.Extent
		if len(exceptionTypes) > 0 {
			endErrorStatement = exceptionTypes[len(exceptionTypes)-1].Extent()
		}
		p.reportIncompleteInput(after(endErrorStatement), "MissingCatchHandlerBlock")
	}
	return newCatchClauseAst(extentOf(catchToken.Extent, handler.Extent()), exceptionTypes, handler)
}

// tryStatementRule is TryStatementRule.
func (p *parser) tryStatementRule(tryToken *Token) StatementAst {
	p.skipNewlines()
	body := p.statementBlockRule()
	if body == nil {
		p.reportIncompleteInput(after(tryToken.Extent), "MissingTryStatementBlock")
	}

	var catches []*CatchClauseAst
	for c := p.catchBlockRule(); c != nil; c = p.catchBlockRule() {
		catches = append(catches, c)
	}

	p.skipNewlines()
	var finallyBlock *StatementBlockAst
	if finallyToken := p.peekToken(); finallyToken.Kind == Finally {
		p.skipToken()
		finallyBlock = p.statementBlockRule()
		if finallyBlock == nil {
			p.reportIncompleteInput(after(finallyToken.Extent), "MissingFinallyStatementBlock", finallyToken.Kind.Text())
		}
	}

	if len(catches) == 0 && finallyBlock == nil {
		p.reportIncompleteInput(after(body.Extent()), "MissingCatchOrFinally")
	}

	var end Extent
	if finallyBlock != nil {
		end = finallyBlock.Extent()
	} else {
		end = catches[len(catches)-1].Extent()
	}
	return newTryStatementAst(extentOf(tryToken.Extent, end), body, catches, finallyBlock)
}

// dataStatementRule is DataStatementRule: data [name] [-SupportedCommand cmd, ...] { ... }.
func (p *parser) dataStatementRule(dataToken *Token) StatementAst {
	p.skipNewlines()
	dataVariableNameAst, _ := p.simpleNameRule()
	dataVariableName := ""
	if dataVariableNameAst != nil {
		dataVariableName = dataVariableNameAst.Value
	}

	p.skipNewlines()
	var commands []ExpressionAst
	if supportedCommandToken := p.peekToken(); supportedCommandToken.Kind == Parameter {
		p.skipToken()
		if !isSpecificParameter(supportedCommandToken, "SupportedCommand") {
			p.reportError(supportedCommandToken.Extent, "InvalidParameterForDataSectionStatement", supportedCommandToken.ParameterName)
		}

		var commaToken *Token
		commands = []ExpressionAst{}
		for {
			p.skipNewlines()
			ast := p.getSingleCommandArgument(argCommandName)
			if ast == nil {
				errAfter := supportedCommandToken.Extent
				if commaToken != nil {
					errAfter = commaToken.Extent
				}
				p.reportIncompleteInput(after(errAfter), "MissingValueForSupportedCommandInDataSectionStatement")
			}
			commands = append(commands, ast)

			commaToken = p.peekToken()
			if commaToken.Kind != Comma {
				break
			}
			p.skipToken()
		}
	}

	body := p.statementBlockRule()
	if body == nil {
		var endErrorStatement Extent
		switch {
		case commands != nil:
			endErrorStatement = commands[len(commands)-1].Extent()
		case dataVariableNameAst != nil:
			endErrorStatement = dataVariableNameAst.Extent()
		default:
			endErrorStatement = dataToken.Extent
		}
		p.reportIncompleteInput(after(endErrorStatement), "MissingStatementBlockForDataSection")
	}

	return newDataStatementAst(extentOf(dataToken.Extent, body.Extent()), dataVariableName, commands, body)
}
