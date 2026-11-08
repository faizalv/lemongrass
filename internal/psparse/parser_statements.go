// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// The statement half of Parser.cs: script blocks and named blocks, statement lists and statements. Pipelines and
// commands are in parser_stmt_pipeline.go; functions, trap, try, data and workflow blocks in
// parser_stmt_function.go. Each method's doc names the C# rule it ports; an out or ref parameter there becomes a
// result or a pointer here. C#'s null extent is emptyExtent, which extentOf already treats as absent. Error
// recovery is left out: reportError stops the parse, so every branch that only builds an ErrorStatementAst after
// reporting is gone.

// scriptBlockRule is ScriptBlockRule; lCurly is nil for a whole script. The caller has set Command mode.
func (p *parser) scriptBlockRule(lCurly *Token, isFilter bool, predefined StatementAst) *ScriptBlockAst {
	p.enter()
	defer p.leave()

	// Skipping newlines first saves a resync when there is no param block.
	p.skipNewlines()
	var usingStatements []*UsingStatementAst
	if lCurly == nil {
		usingStatements = p.usingStatementsRule()
	}
	restorePoint := p.tok.getRestorePoint()
	paramBlock := p.paramBlockRule()
	if paramBlock == nil {
		// Attributes or type constraints scanned for a param block that is not there mean something else, such
		// as a cast, so they are scanned again.
		p.resync(restorePoint)
	}
	p.skipNewlinesAndSemicolons()
	return p.scriptBlockBodyRule(lCurly, usingStatements, paramBlock, isFilter, predefined)
}

// completeScriptBlockBody is CompleteScriptBlockBody: it consumes the closing brace of a script block, or checks
// that a whole script ends here, and returns the script block's full extent. C# returns false only after an
// error, which stops the parse here.
func (p *parser) completeScriptBlockBody(lCurly *Token, bodyExtent *Extent) Extent {
	if lCurly == nil {
		// Tokenizer.GetScriptExtent: the whole input.
		full := Extent{Start: 0, End: len(p.src.text)}
		if t := p.nextToken(); t.Kind != EndOfInput {
			p.reportError(t.Extent, "UnexpectedToken", t.Text)
		}
		return full
	}
	rCurly := p.nextToken()
	if rCurly.Kind != RCurly {
		p.ungetToken(rCurly)
		p.reportIncompleteInputAt(lCurly.Extent, rCurly.Extent, "MissingEndCurlyBrace")
	}
	// An empty body takes the extent between the braces when they are not adjacent. C# compares columns, not
	// offsets, so braces on different lines in the same column count as adjacent.
	if *bodyExtent == emptyExtent && p.src.endColumn(lCurly.Extent) != p.src.startColumn(rCurly.Extent) {
		*bodyExtent = extentOf(after(lCurly.Extent), before(rCurly.Extent))
	}
	return extentOf(lCurly.Extent, rCurly.Extent)
}

// scriptBlockBodyRule is ScriptBlockBodyRule: named blocks, or statements that form an unnamed block.
func (p *parser) scriptBlockBodyRule(lCurly *Token, usingStatements []*UsingStatementAst, paramBlock *ParamBlockAst, isFilter bool, predefined StatementAst) *ScriptBlockAst {
	if p.peekToken().Flags&FlagScriptBlockBlockName != 0 {
		return p.namedBlockListRule(lCurly, usingStatements, paramBlock)
	}

	var traps []*TrapStatementAst
	var statements []StatementAst
	if predefined != nil {
		statements = append(statements, predefined)
	}
	statementListExtent := emptyExtent
	if paramBlock != nil {
		statementListExtent = paramBlock.Extent()
	}
	statementListExtent = extentOf(statementListExtent, p.statementListRule(&statements, &traps))
	scriptBlockExtent := p.completeScriptBlockBody(lCurly, &statementListExtent)
	return newScriptBlockAst(scriptBlockExtent, usingStatements, nil, paramBlock, newStatementBlockAst(statementListExtent, statements, traps), isFilter)
}

// namedBlockListRule is NamedBlockListRule: begin, process, end, clean and dynamicparam blocks.
func (p *parser) namedBlockListRule(lCurly *Token, usingStatements []*UsingStatementAst, paramBlock *ParamBlockAst) *ScriptBlockAst {
	var dynamicParamBlock, beginBlock, processBlock, endBlock, cleanBlock *NamedBlockAst

	startExtent := emptyExtent
	if lCurly != nil {
		startExtent = lCurly.Extent
	} else if paramBlock != nil {
		startExtent = paramBlock.Extent()
	}
	endExtent := emptyExtent
	var extent Extent

blocks:
	for {
		blockNameToken := p.nextToken()
		switch blockNameToken.Kind {
		case RCurly, EndOfInput:
			// completeScriptBlockBody handles the closing brace or end of input.
			p.ungetToken(blockNameToken)
			extent = extentOf(startExtent, endExtent)
			break blocks
		case Dynamicparam, Begin, Process, End, Clean:
		default:
			p.reportError(blockNameToken.Extent, "MissingNamedBlocks", blockNameToken.Text)
		}

		if startExtent == emptyExtent {
			startExtent = blockNameToken.Extent
		}
		statementBlock := p.statementBlockRule()
		if statementBlock == nil {
			p.reportIncompleteInput(after(blockNameToken.Extent), "MissingNamedStatementBlock", blockNameToken.Kind.Text())
		}
		endExtent = statementBlock.Extent()

		extent = extentOf(blockNameToken.Extent, endExtent)
		switch {
		case blockNameToken.Kind == Begin && beginBlock == nil:
			beginBlock = newNamedBlockAst(extent, Begin, statementBlock, false)
		case blockNameToken.Kind == Process && processBlock == nil:
			processBlock = newNamedBlockAst(extent, Process, statementBlock, false)
		case blockNameToken.Kind == End && endBlock == nil:
			endBlock = newNamedBlockAst(extent, End, statementBlock, false)
		case blockNameToken.Kind == Clean && cleanBlock == nil:
			cleanBlock = newNamedBlockAst(extent, Clean, statementBlock, false)
		case blockNameToken.Kind == Dynamicparam && dynamicParamBlock == nil:
			dynamicParamBlock = newNamedBlockAst(extent, Dynamicparam, statementBlock, false)
		default:
			p.reportError(extent, "DuplicateScriptCommandClause", blockNameToken.Kind.Text())
		}

		p.skipNewlinesAndSemicolons()
	}

	scriptBlockExtent := p.completeScriptBlockBody(lCurly, &extent)
	return newScriptBlockAstNamed(scriptBlockExtent, usingStatements, nil, paramBlock, beginBlock, processBlock, endBlock, cleanBlock, dynamicParamBlock)
}

// statementBlockRule is StatementBlockRule: statements in braces. It returns nil when no brace follows.
func (p *parser) statementBlockRule() *StatementBlockAst {
	p.skipNewlines()
	lCurly := p.nextToken()
	if lCurly.Kind != LCurly {
		p.ungetToken(lCurly)
		return nil
	}

	var traps []*TrapStatementAst
	var statements []StatementAst
	p.statementListRule(&statements, &traps)

	rCurly := p.nextToken()
	if rCurly.Kind != RCurly {
		p.ungetToken(rCurly)
		p.reportIncompleteInputAt(lCurly.Extent, rCurly.Extent, "MissingEndCurlyBrace")
	}
	return newStatementBlockAst(extentOf(lCurly.Extent, rCurly.Extent), statements, traps)
}

// statementListRule is StatementListRule; it returns the extent the statements cover, emptyExtent for none.
func (p *parser) statementListRule(statements *[]StatementAst, traps *[]*TrapStatementAst) Extent {
	p.enter()
	defer p.leave()

	var first, last StatementAst
	p.skipNewlinesAndSemicolons()
	for {
		statement := p.statementRule()
		if statement == nil {
			break
		}
		p.tok.checkAstIsBeforeSignature(statement)

		if trap, ok := statement.(*TrapStatementAst); ok {
			*traps = append(*traps, trap)
		} else {
			*statements = append(*statements, statement)
		}
		if first == nil {
			first = statement
		}
		last = statement

		p.skipNewlinesAndSemicolons()
		if k := p.peekToken().Kind; k == RParen || k == RCurly {
			break
		}
	}
	if first == nil {
		return emptyExtent
	}
	return extentOf(first.Extent(), last.Extent())
}

// statementRule is StatementRule. It returns nil only at the end of input. DSC configuration and dynamic
// keywords report ErrDynamicKeywords (contract decision 5).
func (p *parser) statementRule() StatementAst {
	p.enter()
	defer p.leave()

	restorePoint := 0
	t := p.nextToken()

	var attributes []AttributeBaseAst
	if t.Kind == Generic && len(t.Text) > 0 && t.Text[0] == '[' {
		restorePoint = t.Extent.Start
		p.resyncToken(t)
		attributes = p.attributeListRule(false)
		t = p.nextToken()

		if len(attributes) > 0 {
			switch {
			case t.Flags&FlagStatementDoesntSupportAttributes != 0:
				// A type constraint before such a statement is a cast in an expression, so it is scanned again.
				if hasTypeConstraint(attributes) {
					p.resync(restorePoint)
					t = p.nextToken()
				} else {
					p.reportError(attributes[0].Extent(), "UnexpectedAttribute", attributes[0].GetTypeName().FullName())
				}
			case t.Flags&FlagKeyword != 0:
				for _, attr := range attributes {
					if _, ok := attr.(*AttributeAst); !ok {
						p.reportError(attr.Extent(), "TypeNotAllowedBeforeStatement", attr.GetTypeName().FullName())
					}
				}
			default:
				p.resync(restorePoint)
				t = p.nextToken()
			}
		}
	}

	var statement StatementAst
	switch t.Kind {
	case If:
		statement = p.ifStatementRule(t)
	case Switch:
		statement = p.switchStatementRule(nil, t)
	case Foreach:
		statement = p.foreachStatementRule(nil, t)
	case For:
		statement = p.forStatementRule(nil, t)
	case While:
		statement = p.whileStatementRule(nil, t)
	case Do:
		statement = p.doWhileStatementRule(nil, t)
	case Function, Filter, Workflow:
		statement = p.functionDeclarationRule(t)
	case Return:
		statement = p.returnStatementRule(t)
	case Throw:
		statement = p.throwStatementRule(t)
	case Exit:
		statement = p.exitStatementRule(t)
	case Break:
		statement = p.breakStatementRule(t)
	case Continue:
		statement = p.continueStatementRule(t)
	case Trap:
		statement = p.trapStatementRule(t)
	case Try:
		statement = p.tryStatementRule(t)
	case Data:
		statement = p.dataStatementRule(t)
	case Parallel, Sequence:
		statement = p.blockStatementRule(t)
	case Configuration, DynamicKeyword:
		p.reportError(t.Extent, ErrDynamicKeywords)
	case From, Define, Var:
		p.reportError(t.Extent, "ReservedKeywordNotAllowed", t.Kind.Text())
	case Label:
		p.skipNewlines()
		statement = p.labeledStatementRule(t)
	case EndOfInput:
		if attributes != nil {
			p.resync(restorePoint)
			statement = p.pipelineChainRule()
		} else {
			p.ungetToken(t)
			statement = nil
		}
	case Class:
		statement = p.classDefinitionRule(attributes, t)
	case Enum:
		statement = p.enumDefinitionRule(attributes, t)
	case Using:
		statement = p.usingStatementRule(t)
		// Using statements belong before anything else in a script; C# parses one here only to report it.
		p.reportError(statement.Extent(), "UsingMustBeAtStartOfScript")
	default:
		// else, elseif, catch and until land here too: C# skips them only after an earlier error.
		if attributes != nil {
			p.resync(restorePoint)
		} else {
			p.ungetToken(t)
		}
		statement = p.pipelineChainRule()
	}
	return statement
}

// hasTypeConstraint is attributes.OfType<TypeConstraintAst>().Any().
func hasTypeConstraint(attributes []AttributeBaseAst) bool {
	for _, a := range attributes {
		if _, ok := a.(*TypeConstraintAst); ok {
			return true
		}
	}
	return false
}

// simpleNameRule is SimpleNameRule(out Token): a bare identifier, as a loop label, data variable or member name.
// The token is returned whether or not it is one.
func (p *parser) simpleNameRule() (*StringConstantExpressionAst, *Token) {
	p.tok.WantSimpleName = true
	t := p.peekToken()
	p.tok.WantSimpleName = false

	if t.Kind == Identifier {
		// Marked as a member name even for a label or data variable name, as PowerShell 2 did.
		t.Flags |= FlagMemberName
		p.skipToken()
		return newStringConstantExpressionAst(t.Extent, t.Text, BareWord), t
	}
	return nil, t
}

// labelOrKeyRule is LabelOrKeyRule: a simple name or a unary expression, for a break or continue label and a
// hashtable key.
func (p *parser) labelOrKeyRule() ExpressionAst {
	if simpleName, _ := p.simpleNameRule(); simpleName != nil {
		return simpleName
	}
	if t := p.peekToken(); t.Kind != NewLine && t.Kind != Semi {
		disableCommaOperator := p.disableCommaOperator
		p.disableCommaOperator = true
		labelExpr := p.unaryExpressionRule(false)
		p.disableCommaOperator = disableCommaOperator
		if labelExpr != nil {
			return labelExpr
		}
	}
	return nil
}

// breakStatementRule is BreakStatementRule.
func (p *parser) breakStatementRule(breakToken *Token) *BreakStatementAst {
	labelExpr := p.labelOrKeyRule()
	extent := breakToken.Extent
	if labelExpr != nil {
		extent = extentOf(breakToken.Extent, labelExpr.Extent())
	}
	return newBreakStatementAst(extent, labelExpr)
}

// continueStatementRule is ContinueStatementRule.
func (p *parser) continueStatementRule(continueToken *Token) *ContinueStatementAst {
	labelExpr := p.labelOrKeyRule()
	extent := continueToken.Extent
	if labelExpr != nil {
		extent = extentOf(continueToken.Extent, labelExpr.Extent())
	}
	return newContinueStatementAst(extent, labelExpr)
}

// flowPipeline reads the optional pipeline chain of return, exit and throw and the statement's extent.
func (p *parser) flowPipeline(t *Token) (PipelineBaseAst, Extent) {
	pipeline := p.pipelineChainRule()
	if pipeline == nil {
		return nil, t.Extent
	}
	return pipeline, extentOf(t.Extent, pipeline.Extent())
}

// returnStatementRule is ReturnStatementRule.
func (p *parser) returnStatementRule(t *Token) *ReturnStatementAst {
	pipeline, extent := p.flowPipeline(t)
	return newReturnStatementAst(extent, pipeline)
}

// exitStatementRule is ExitStatementRule.
func (p *parser) exitStatementRule(t *Token) *ExitStatementAst {
	pipeline, extent := p.flowPipeline(t)
	return newExitStatementAst(extent, pipeline)
}

// throwStatementRule is ThrowStatementRule.
func (p *parser) throwStatementRule(t *Token) *ThrowStatementAst {
	pipeline, extent := p.flowPipeline(t)
	return newThrowStatementAst(extent, pipeline)
}

// labeledStatementRule is LabeledStatementRule: a loop or switch after a :label, or else a pipeline that starts
// with the label text.
func (p *parser) labeledStatementRule(label *Token) StatementAst {
	t := p.nextToken()
	switch t.Kind {
	case Switch:
		return p.switchStatementRule(label, t)
	case Foreach:
		return p.foreachStatementRule(label, t)
	case For:
		return p.forStatementRule(label, t)
	case While:
		return p.whileStatementRule(label, t)
	case Do:
		return p.doWhileStatementRule(label, t)
	}
	// Two tokens would need pushing back, and only one can be, so rescan from the label.
	p.resyncToken(label)
	return p.pipelineChainRule()
}

// labelText is labelToken?.LabelText.
func labelText(labelToken *Token) string {
	if labelToken == nil {
		return ""
	}
	return labelToken.StringValue
}

// labelOr is (labelToken ?? keywordToken).Extent.
func labelOr(labelToken, keywordToken *Token) Extent {
	if labelToken != nil {
		return labelToken.Extent
	}
	return keywordToken.Extent
}

// ifStatementRule is IfStatementRule.
func (p *parser) ifStatementRule(ifToken *Token) StatementAst {
	var clauses []IfClause
	var elseClause *StatementBlockAst
	keyword := ifToken

	for {
		p.skipNewlines()
		lParen := p.nextToken()
		if lParen.Kind != LParen {
			p.ungetToken(lParen)
			p.reportIncompleteInput(after(keyword.Extent), "MissingOpenParenthesisInIfStatement", keyword.Text)
		}

		p.skipNewlines()
		condition := p.pipelineChainRule()
		if condition == nil {
			p.reportIncompleteInput(after(lParen.Extent), "IfStatementMissingCondition", keyword.Text)
		}

		p.skipNewlines()
		rParen := p.nextToken()
		if rParen.Kind != RParen {
			p.ungetToken(rParen)
			p.reportIncompleteInput(rParen.Extent, "MissingEndParenthesisAfterStatement", keyword.Text)
		}

		p.skipNewlines()
		body := p.statementBlockRule()
		if body == nil {
			p.reportIncompleteInput(rParen.Extent, "MissingStatementBlock", keyword.Text)
		}
		clauses = append(clauses, IfClause{Condition: condition, Body: body})

		// With no elseif or else after it, the newlines skipped here are scanned again: a key after an if in a
		// hashtable needs the newline before it.
		restorePoint := 0
		if p.ungotToken == nil {
			restorePoint = p.tok.getRestorePoint()
		} else {
			restorePoint = p.ungotToken.Extent.Start
		}
		p.skipNewlines()
		keyword = p.peekToken()

		if keyword.Kind == ElseIf {
			p.skipToken()
			continue
		}
		if keyword.Kind == Else {
			p.skipToken()
			p.skipNewlines()
			elseClause = p.statementBlockRule()
			if elseClause == nil {
				p.reportIncompleteInput(after(keyword.Extent), "MissingStatementBlockAfterElse")
			}
		} else {
			p.resync(restorePoint)
		}
		break
	}

	endExtent := clauses[len(clauses)-1].Body.Extent()
	if elseClause != nil {
		endExtent = elseClause.Extent()
	}
	return newIfStatementAst(extentOf(ifToken.Extent, endExtent), clauses, elseClause)
}

// switchStatementRule is SwitchStatementRule; labelToken may be nil.
func (p *parser) switchStatementRule(labelToken, switchToken *Token) StatementAst {
	endErrorStatement := labelOr(labelToken, switchToken)
	p.skipNewlines()

	var condition PipelineBaseAst
	var flags SwitchFlags
	switchParameterToken := p.peekToken()
	for switchParameterToken.Kind == Parameter {
		p.skipToken()
		endErrorStatement = switchParameterToken.Extent

		switch {
		case isSpecificParameter(switchParameterToken, "regex"):
			flags |= SwitchRegex
			flags &^= SwitchWildcard
		case isSpecificParameter(switchParameterToken, "wildcard"):
			flags |= SwitchWildcard
			flags &^= SwitchRegex
		case isSpecificParameter(switchParameterToken, "exact"):
			flags &^= SwitchRegex
			flags &^= SwitchWildcard
		case isSpecificParameter(switchParameterToken, "casesensitive"):
			flags |= SwitchCaseSensitive
		case isSpecificParameter(switchParameterToken, "parallel"):
			flags |= SwitchParallel
		case isSpecificParameter(switchParameterToken, "file"):
			flags |= SwitchFile
			p.skipNewlines()
			fileNameExpr := p.getSingleCommandArgument(argFileName)
			if fileNameExpr == nil {
				p.reportIncompleteInput(after(switchParameterToken.Extent), "MissingFilenameOption")
			}
			endErrorStatement = fileNameExpr.Extent()
			condition = newPipelineAst(fileNameExpr.Extent(), []CommandBaseAst{newCommandExpressionAst(fileNameExpr.Extent(), fileNameExpr, nil)}, false)
		default:
			p.reportError(switchParameterToken.Extent, "InvalidSwitchFlag", switchParameterToken.ParameterName)
		}

		switchParameterToken = p.peekToken()
	}

	if lParen := p.peekToken(); lParen.Kind == LParen {
		endErrorStatement = lParen.Extent
		p.skipToken()
		if flags&SwitchFile != 0 {
			p.reportError(lParen.Extent, "PipelineValueRequired")
		}

		p.skipNewlines()
		condition = p.pipelineChainRule()
		if condition == nil {
			p.reportIncompleteInput(after(lParen.Extent), "PipelineValueRequired")
		}
		endErrorStatement = condition.Extent()

		p.skipNewlines()
		if rParen := p.nextToken(); rParen.Kind != RParen {
			p.ungetToken(rParen)
			p.reportIncompleteInput(after(endErrorStatement), "MissingEndParenthesisInSwitchStatement")
		}
	} else if condition == nil {
		// -file without a file name has already reported, so only a missing condition is left.
		p.reportIncompleteInput(after(endErrorStatement), "PipelineValueRequired")
	}

	p.skipNewlines()
	lCurly := p.nextToken()
	if lCurly.Kind != LCurly {
		p.ungetToken(lCurly)
		p.reportIncompleteInput(after(endErrorStatement), "MissingCurlyBraceInSwitchStatement")
	}
	endErrorStatement = lCurly.Extent
	p.skipNewlines()

	var def *StatementBlockAst
	var clauses []SwitchClause
	var rCurly *Token
	for {
		t := p.peekToken()
		isDefaultClause := t.Kind == Default
		var clauseCondition ExpressionAst
		if isDefaultClause {
			p.skipToken()
			clauseCondition = newStringConstantExpressionAst(t.Extent, t.Text, BareWord)
		} else {
			clauseCondition = p.getSingleCommandArgument(argSwitchCondition)
			if clauseCondition == nil {
				p.reportIncompleteInput(after(endErrorStatement), "MissingSwitchConditionExpression")
			}
		}
		endErrorStatement = clauseCondition.Extent()

		clauseBody := p.statementBlockRule()
		if clauseBody == nil {
			p.reportIncompleteInput(after(endErrorStatement), "MissingSwitchStatementClause")
		}
		endErrorStatement = clauseBody.Extent()
		if isDefaultClause {
			if def != nil {
				p.reportError(clauseCondition.Extent(), "MultipleSwitchDefaultClauses")
			}
			def = clauseBody
		} else {
			clauses = append(clauses, SwitchClause{Condition: clauseCondition, Body: clauseBody})
		}

		p.skipNewlinesAndSemicolons()
		t = p.peekToken()
		if t.Kind == RCurly {
			rCurly = t
			p.skipToken()
			break
		}
		if t.Kind == EndOfInput {
			p.reportIncompleteInputAt(lCurly.Extent, t.Extent, "MissingEndCurlyBrace")
		}
	}

	return newSwitchStatementAst(extentOf(labelOr(labelToken, switchToken), rCurly.Extent), labelText(labelToken), condition, flags, clauses, def)
}

// getWordOrExpression is GetWordOrExpression: the argument after a keyword such as a DSC resource name.
func (p *parser) getWordOrExpression(keywordToken *Token) ExpressionAst {
	nameToken := p.nextToken()
	if nameToken.Kind == EndOfInput {
		p.ungetToken(nameToken)
		p.reportIncompleteInput(after(keywordToken.Extent), "RequiredNameOrExpressionMissing")
	}
	argument := p.getCommandArgument(argCommandArgument, nameToken)
	if argument == nil {
		p.reportError(after(keywordToken.Extent), "ParameterRequiresArgument", keywordToken.Text)
	}
	return argument
}

// foreachStatementRule is ForeachStatementRule.
func (p *parser) foreachStatementRule(labelToken, forEachToken *Token) StatementAst {
	startOfStatement := labelOr(labelToken, forEachToken)
	p.skipNewlines()

	var flags ForEachFlags
	var throttleLimit ExpressionAst
	foreachParameterToken := p.peekToken()
	for foreachParameterToken.Kind == Parameter {
		p.skipToken()
		switch {
		case isSpecificParameter(foreachParameterToken, "parallel"):
			flags |= ForEachParallel
		case isSpecificParameter(foreachParameterToken, "throttlelimit"):
			p.skipNewlines()
			throttleLimit = p.getSingleCommandArgument(argCommandArgument)
			if throttleLimit == nil {
				p.reportIncompleteInput(after(foreachParameterToken.Extent), "MissingThrottleLimit")
			}
		default:
			p.reportError(foreachParameterToken.Extent, "InvalidForeachFlag", foreachParameterToken.ParameterName)
		}
		p.skipNewlines()
		foreachParameterToken = p.peekToken()
	}

	lParen := p.nextToken()
	if lParen.Kind != LParen {
		p.ungetToken(lParen)
		p.reportIncompleteInput(after(forEachToken.Extent), "MissingOpenParenthesisAfterKeyword", forEachToken.Kind.Text())
	}
	p.skipNewlines()

	t := p.nextToken()
	if t.Kind != Variable && t.Kind != SplattedVariable {
		p.ungetToken(t)
		p.reportIncompleteInput(after(lParen.Extent), "MissingVariableNameAfterForeach")
	}
	variableAst := newVariableExpressionAst(t)
	p.skipNewlines()

	inToken := p.nextToken()
	if inToken.Kind != In {
		p.ungetToken(inToken)
		p.reportIncompleteInput(after(variableAst.Extent()), "MissingInInForeach")
	}
	p.skipNewlines()
	pipeline := p.pipelineChainRule()
	if pipeline == nil {
		p.reportIncompleteInput(after(inToken.Extent), "MissingForeachExpression")
	}
	p.skipNewlines()
	rParen := p.nextToken()
	if rParen.Kind != RParen {
		p.ungetToken(rParen)
		p.reportIncompleteInput(after(pipeline.Extent()), "MissingEndParenthesisAfterForeach")
	}
	body := p.statementBlockRule()
	if body == nil {
		p.reportIncompleteInput(after(rParen.Extent), "MissingForeachStatement")
	}

	return newForEachStatementAst(extentOf(startOfStatement, body.Extent()), labelText(labelToken), flags, throttleLimit, variableAst, pipeline, body)
}

// forStatementRule is ForStatementRule.
func (p *parser) forStatementRule(labelToken, forToken *Token) StatementAst {
	endErrorStatement := emptyExtent
	p.skipNewlines()
	lParen := p.nextToken()
	if lParen.Kind != LParen {
		p.ungetToken(lParen)
		p.reportIncompleteInput(after(forToken.Extent), "MissingOpenParenthesisAfterKeyword", forToken.Kind.Text())
	}

	p.skipNewlines()
	initializer := p.pipelineChainRule()
	if initializer != nil {
		endErrorStatement = initializer.Extent()
	}
	if p.peekToken().Kind == Semi {
		endErrorStatement = p.nextToken().Extent
	}

	p.skipNewlines()
	condition := p.pipelineChainRule()
	if condition != nil {
		endErrorStatement = condition.Extent()
	}
	if p.peekToken().Kind == Semi {
		endErrorStatement = p.nextToken().Extent
	}

	p.skipNewlines()
	iterator := p.pipelineChainRule()
	if iterator != nil {
		endErrorStatement = iterator.Extent()
	}

	p.skipNewlines()
	rParen := p.nextToken()
	if rParen.Kind != RParen {
		p.ungetToken(rParen)
		if endErrorStatement == emptyExtent {
			endErrorStatement = lParen.Extent
		}
		p.reportIncompleteInput(after(endErrorStatement), "MissingEndParenthesisAfterStatement", forToken.Kind.Text())
	}
	body := p.statementBlockRule()
	if body == nil {
		p.reportIncompleteInput(after(rParen.Extent), "MissingLoopStatement", forToken.Kind.Text())
	}

	return newForStatementAst(extentOf(labelOr(labelToken, forToken), body.Extent()), labelText(labelToken), initializer, condition, iterator, body)
}

// whileStatementRule is WhileStatementRule.
func (p *parser) whileStatementRule(labelToken, whileToken *Token) StatementAst {
	p.skipNewlines()
	lParen := p.nextToken()
	if lParen.Kind != LParen {
		p.ungetToken(lParen)
		p.reportIncompleteInput(after(whileToken.Extent), "MissingOpenParenthesisAfterKeyword", whileToken.Text)
	}

	p.skipNewlines()
	condition := p.pipelineChainRule()
	if condition == nil {
		p.reportIncompleteInput(after(lParen.Extent), "MissingExpressionAfterKeyword", whileToken.Kind.Text())
	}

	p.skipNewlines()
	rParen := p.nextToken()
	if rParen.Kind != RParen {
		p.ungetToken(rParen)
		p.reportIncompleteInput(after(condition.Extent()), "MissingEndParenthesisAfterStatement", whileToken.Kind.Text())
	}

	p.skipNewlines()
	body := p.statementBlockRule()
	if body == nil {
		p.reportIncompleteInput(after(rParen.Extent), "MissingLoopStatement", whileToken.Kind.Text())
	}

	return newWhileStatementAst(extentOf(labelOr(labelToken, whileToken), body.Extent()), labelText(labelToken), condition, body)
}

// doWhileStatementRule is DoWhileStatementRule (do-while and do-until).
func (p *parser) doWhileStatementRule(labelToken, doToken *Token) StatementAst {
	startExtent := labelOr(labelToken, doToken)

	body := p.statementBlockRule()
	if body == nil {
		p.reportIncompleteInput(after(doToken.Extent), "MissingLoopStatement", Do.Text())
	}

	p.skipNewlines()
	whileOrUntilToken := p.nextToken()
	if whileOrUntilToken.Kind != While && whileOrUntilToken.Kind != Until {
		p.ungetToken(whileOrUntilToken)
		p.reportIncompleteInput(after(body.Extent()), "MissingWhileOrUntilInDoWhile")
	}

	p.skipNewlines()
	lParen := p.nextToken()
	if lParen.Kind != LParen {
		p.ungetToken(lParen)
		p.reportIncompleteInput(after(whileOrUntilToken.Extent), "MissingOpenParenthesisAfterKeyword", whileOrUntilToken.Kind.Text())
	}

	p.skipNewlines()
	condition := p.pipelineChainRule()
	if condition == nil {
		p.reportIncompleteInput(after(lParen.Extent), "MissingExpressionAfterKeyword", whileOrUntilToken.Kind.Text())
	}

	p.skipNewlines()
	rParen := p.nextToken()
	if rParen.Kind != RParen {
		p.ungetToken(rParen)
		p.reportIncompleteInput(after(condition.Extent()), "MissingEndParenthesisAfterStatement", whileOrUntilToken.Kind.Text())
	}

	extent := extentOf(startExtent, rParen.Extent)
	if whileOrUntilToken.Kind == Until {
		return newDoUntilStatementAst(extent, labelText(labelToken), condition, body)
	}
	return newDoWhileStatementAst(extent, labelText(labelToken), condition, body)
}
