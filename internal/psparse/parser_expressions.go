// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// The expression half of Parser.cs, core rules: ternary, binary, array literal, unary and primary expressions,
// hashtables, script block, sub- and parenthesized expressions. Member access lives in parser_expr_member.go,
// expandable strings in parser_expr_string.go, param blocks and attributes in parser_expr_param.go, class, enum
// and using in parser_expr_class.go, type names in parser_types.go.
//
// Each method's doc names the C# rule it ports; an out or ref parameter there becomes a result or a pointer here.
// Error recovery branches are left out: reportError stops the parse, so only the report before each recovery
// survives. A tokenizer mode or flag a rule changes is restored on every normal return, as the C# finally blocks
// do; after an error nothing is restored, since the parse is over. A rule returning no expression returns a nil
// interface, never a typed nil pointer, so callers can test it against nil.

// expressionRule is ExpressionRule: a binary expression, or a ternary built from them.
func (p *parser) expressionRule(endNumberOnTernaryOpChars bool) ExpressionAst {
	p.enter()
	defer p.leave()
	oldMode := p.tok.Mode
	p.setTokenizerMode(ModeExpression)
	expr := p.ternaryExpression(endNumberOnTernaryOpChars)
	p.setTokenizerMode(oldMode)
	return expr
}

// ternaryExpression is the body of ExpressionRule inside its tokenizer mode switch.
func (p *parser) ternaryExpression(endNumberOnTernaryOpChars bool) ExpressionAst {
	condition := p.binaryExpressionRule(endNumberOnTernaryOpChars)
	if condition == nil {
		return nil
	}
	t := p.peekToken()
	if t.Kind != QuestionMark {
		return condition
	}
	p.skipToken()
	p.skipNewlines()

	// After '?' the operands are expressions for sure, so a number ends at '?' and ':'.
	ifTrue := p.expressionRule(true)
	if ifTrue == nil {
		p.reportIncompleteInput(after(t.Extent), "ExpectedValueExpression", t.Text)
	}
	p.skipNewlines()
	t = p.nextToken()
	if t.Kind != Colon {
		p.ungetToken(t)
		p.reportIncompleteInput(t.Extent, "MissingColonInTernaryExpression")
	}
	p.skipNewlines()
	ifFalse := p.expressionRule(true)
	if ifFalse == nil {
		p.reportIncompleteInput(after(t.Extent), "ExpectedValueExpression", t.Text)
	}
	return newTernaryExpressionAst(extentOf(condition.Extent(), ifFalse.Extent()), condition, ifTrue, ifFalse)
}

// binaryExpressionRule is BinaryExpressionRule: operator precedence parsing over array literal operands.
func (p *parser) binaryExpressionRule(endNumberOnTernaryOpChars bool) ExpressionAst {
	p.enter()
	defer p.leave()
	oldMode := p.tok.Mode
	p.setTokenizerMode(ModeExpression)
	expr := p.binaryExpression(endNumberOnTernaryOpChars)
	p.setTokenizerMode(oldMode)
	return expr
}

// binaryExpression is the body of BinaryExpressionRule inside its tokenizer mode switch.
func (p *parser) binaryExpression(endNumberOnTernaryOpChars bool) ExpressionAst {
	expr := p.arrayLiteralRule(endNumberOnTernaryOpChars)
	if expr == nil {
		return nil
	}
	t := p.peekToken()
	if !t.Kind.HasTrait(FlagBinaryOperator) {
		if t.Kind == Parameter {
			p.errorRecoveryParameterInExpression(t)
		}
		return expr
	}
	if t.Kind == AndAnd || t.Kind == OrOr {
		return expr
	}
	p.skipToken()

	operands := []ExpressionAst{expr}
	operators := []*Token{t}
	precedence := t.Kind.BinaryPrecedence()
	for {
		p.skipNewlines()
		// After a binary operator the right-hand side is an expression for sure.
		expr = p.arrayLiteralRule(true)
		if expr == nil {
			// token.Text, not the kind's text: a case-insensitive operator may be spelled differently.
			p.reportIncompleteInput(after(t.Extent), "ExpectedValueExpression", t.Text)
		}
		operands = append(operands, expr)

		t = p.nextToken()
		if !t.Kind.HasTrait(FlagBinaryOperator) {
			p.ungetToken(t)
			if t.Kind == Parameter {
				// C# remembers the parameter token here and reports it once the tree is folded; nothing between
				// can fail, so reporting now gives the same first error.
				p.errorRecoveryParameterInExpression(t)
			}
			break
		}

		newPrecedence := t.Kind.BinaryPrecedence()
		for newPrecedence <= precedence {
			rhs := operands[len(operands)-1]
			lhs := operands[len(operands)-2]
			operands = operands[:len(operands)-2]
			op := operators[len(operators)-1]
			operators = operators[:len(operators)-1]
			operands = append(operands, newBinaryExpressionAst(extentOf(lhs.Extent(), rhs.Extent()), lhs, op.Kind, rhs, op.Extent))
			if len(operators) == 0 {
				break
			}
			precedence = operators[len(operators)-1].Kind.BinaryPrecedence()
		}
		operators = append(operators, t)
		precedence = newPrecedence
	}

	rhs := operands[len(operands)-1]
	operands = operands[:len(operands)-1]
	for len(operands) > 0 {
		lhs := operands[len(operands)-1]
		operands = operands[:len(operands)-1]
		op := operators[len(operators)-1]
		operators = operators[:len(operators)-1]
		rhs = newBinaryExpressionAst(extentOf(lhs.Extent(), rhs.Extent()), lhs, op.Kind, rhs, op.Extent)
	}
	return rhs
}

// errorRecoveryParameterInExpression is ErrorRecoveryParameterInExpression: a parameter token after an operand,
// such as -foo in 1 -foo, is an unexpected token.
func (p *parser) errorRecoveryParameterInExpression(paramToken *Token) {
	p.reportError(paramToken.Extent, "UnexpectedToken", paramToken.Text)
}

// arrayLiteralRule is ArrayLiteralRule: unary expressions separated by commas.
func (p *parser) arrayLiteralRule(endNumberOnTernaryOpChars bool) ExpressionAst {
	lastExpr := p.unaryExpressionRule(endNumberOnTernaryOpChars)
	if lastExpr == nil {
		return nil
	}
	firstExpr := lastExpr
	commaToken := p.peekToken()
	if commaToken.Kind != Comma || p.disableCommaOperator {
		return lastExpr
	}

	arrayValues := []ExpressionAst{lastExpr}
	for commaToken.Kind == Comma {
		p.skipToken()
		p.skipNewlines()
		// After a comma an element is an expression for sure.
		lastExpr = p.unaryExpressionRule(true)
		if lastExpr == nil {
			p.reportIncompleteInput(after(commaToken.Extent), "MissingExpressionAfterToken", commaToken.Text)
		}
		arrayValues = append(arrayValues, lastExpr)
		commaToken = p.peekToken()
	}
	return newArrayLiteralAst(extentOf(firstExpr.Extent(), lastExpr.Extent()), arrayValues)
}

// unaryExpressionRule is UnaryExpressionRule: a prefix operator, a cast or attributed expression, or a primary
// expression, each with an optional postfix ++ or --.
func (p *parser) unaryExpressionRule(endNumberOnTernaryOpChars bool) ExpressionAst {
	p.enter()
	defer p.leave()

	oldAllowSignedNumbers := p.tok.AllowSignedNumbers
	oldForceEndNumber := p.tok.ForceEndNumberOnTernaryOpChars
	p.tok.AllowSignedNumbers = true
	p.tok.ForceEndNumberOnTernaryOpChars = endNumberOnTernaryOpChars
	if p.ungotToken != nil {
		// A pushed-back '-' may be the sign of a number, and a pushed-back generic token may be a number glued to
		// ternary operator characters; both rescan under the flags just set.
		needResync := p.ungotToken.Kind == Minus
		if !needResync {
			needResync = endNumberOnTernaryOpChars && p.ungotToken.Kind == Generic
		}
		if needResync {
			p.resyncToken(p.ungotToken)
		}
	}
	t := p.peekToken()
	p.tok.AllowSignedNumbers = oldAllowSignedNumbers
	p.tok.ForceEndNumberOnTernaryOpChars = oldForceEndNumber

	var expr ExpressionAst
	switch {
	case t.Kind.HasTrait(FlagUnaryOperator):
		if p.disableCommaOperator && t.Kind == Comma {
			return nil
		}
		p.skipToken()
		p.skipNewlines()
		child := p.unaryExpressionRule(true)
		if child == nil {
			p.reportIncompleteInput(after(t.Extent), "MissingExpressionAfterOperator", t.Text)
		}
		if t.Kind == Comma {
			expr = newArrayLiteralAst(extentOf(t.Extent, child.Extent()), []ExpressionAst{child})
		} else {
			expr = newUnaryExpressionAst(extentOf(t.Extent, child.Extent()), t.Kind, child)
		}

	case t.Kind == LBracket:
		// A type literal or an attribute.
		attributes := p.attributeListRule(true)
		if attributes == nil {
			return nil
		}
		lastAttribute := attributes[len(attributes)-1]
		if _, ok := lastAttribute.(*AttributeAst); ok {
			p.skipNewlines()
			child := p.unaryExpressionRule(true)
			if child == nil {
				p.reportIncompleteInput(lastAttribute.Extent(), "UnexpectedAttribute", lastAttribute.GetTypeName().FullName())
			}
			expr = newAttributedExpressionAst(extentOf(lastAttribute.Extent(), child.Extent()), lastAttribute, child)
		} else {
			var memberAccessToken *Token
			if p.ungotToken == nil {
				memberAccessToken = p.nextMemberAccessToken(false)
			}
			if memberAccessToken != nil {
				expr = p.checkPostPrimaryExpressionOperators(memberAccessToken,
					newTypeExpressionAst(lastAttribute.Extent(), lastAttribute.GetTypeName()))
			} else {
				t = p.peekToken()
				if t.Kind != NewLine && t.Kind != Comma {
					child := p.unaryExpressionRule(true)
					if child != nil {
						expr = newConvertExpressionAst(extentOf(lastAttribute.Extent(), child.Extent()),
							lastAttribute.(*TypeConstraintAst), child)
					}
				}
			}
			if expr == nil {
				expr = newTypeExpressionAst(lastAttribute.Extent(), lastAttribute.GetTypeName())
			}
		}
		for i := len(attributes) - 2; i >= 0; i-- {
			if tc, ok := attributes[i].(*TypeConstraintAst); ok {
				expr = newConvertExpressionAst(extentOf(tc.Extent(), expr.Extent()), tc, expr)
			} else {
				expr = newAttributedExpressionAst(extentOf(attributes[i].Extent(), expr.Extent()), attributes[i], expr)
			}
		}

	default:
		expr = p.primaryExpressionRule(true)
	}

	if expr != nil {
		t = p.peekToken()
		op := Unknown
		switch t.Kind {
		case PlusPlus:
			op = PostfixPlusPlus
		case MinusMinus:
			op = PostfixMinusMinus
		}
		if op != Unknown {
			p.skipToken()
			expr = newUnaryExpressionAst(extentOf(expr.Extent(), t.Extent), op, expr)
		}
	}
	return expr
}

// primaryExpressionRule is PrimaryExpressionRule: a value, then member and element access when withMemberAccess.
func (p *parser) primaryExpressionRule(withMemberAccess bool) ExpressionAst {
	p.enter()
	defer p.leave()

	var expr ExpressionAst
	t := p.nextToken()
	switch t.Kind {
	case SplattedVariable, Variable:
		expr = p.checkUsingVariable(t, withMemberAccess)
	case Number:
		expr = newConstantExpressionAstFromNumber(t)
	case HereStringExpandable, StringExpandable:
		expr = p.expandableStringRule(t)
	case HereStringLiteral, StringLiteral:
		expr = newStringConstantExpressionAstFromToken(t)
	case LParen:
		expr = p.parenthesizedExpressionRule(t)
	case AtParen, DollarParen:
		expr = p.subExpressionRule(t)
	case AtCurly:
		expr = p.hashExpressionRule(t, false)
	case LCurly:
		expr = p.scriptBlockExpressionRule(t)
	default:
		p.ungetToken(t)
		return nil
	}
	if !withMemberAccess {
		return expr
	}
	return p.checkPostPrimaryExpressionOperators(p.nextMemberAccessToken(true), expr)
}

// checkUsingVariable is CheckUsingVariable: $using:x becomes a UsingExpressionAst around $x, with its member
// accesses inside it.
func (p *parser) checkUsingVariable(variableToken *Token, withMemberAccess bool) ExpressionAst {
	path := variableToken.VariablePath
	if path.IsDriveQualified() && OrdinalEqualFold(path.DriveName(), "using") && path.UnqualifiedPath() != "" {
		realPath := NewVariablePath(path.UnqualifiedPath())
		var child ExpressionAst = newVariableExpressionAstPath(variableToken.Extent, realPath, variableToken.Kind == SplattedVariable)
		if withMemberAccess {
			child = p.checkPostPrimaryExpressionOperators(p.nextMemberAccessToken(true), child)
		}
		return newUsingExpressionAst(child.Extent(), child)
	}
	return newVariableExpressionAst(variableToken)
}

// hashExpressionRule is HashExpressionRule: @{ key = statement; ... }. parsingSchemaElement is for DSC, which
// is not ported, so it only picks the error IDs.
func (p *parser) hashExpressionRule(atCurlyToken *Token, parsingSchemaElement bool) ExpressionAst {
	p.skipNewlines()
	keyValuePairs := []KeyValuePair{}
	for {
		pair := p.getKeyValuePair(parsingSchemaElement)
		if pair == nil {
			break
		}
		keyValuePairs = append(keyValuePairs, *pair)
		t := p.peekToken()
		if t.Kind != NewLine && t.Kind != Semi {
			break
		}
		p.skipNewlinesAndSemicolons()
	}

	rCurly := p.nextToken()
	if rCurly.Kind != RCurly {
		p.ungetToken(rCurly)
		id := "MissingEndCurlyBrace"
		if parsingSchemaElement {
			id = "IncompletePropertyAssignmentBlock"
		}
		p.reportIncompleteInputAt(after(atCurlyToken.Extent), rCurly.Extent, id)
	}
	return newHashtableAst(extentOf(atCurlyToken.Extent, rCurly.Extent), keyValuePairs)
}

// getKeyValuePair is GetKeyValuePair; nil when no entry starts here. The key is read in expression mode and the
// value as a statement in command mode.
func (p *parser) getKeyValuePair(parsingSchemaElement bool) *KeyValuePair {
	oldMode := p.tok.Mode
	p.setTokenizerMode(ModeExpression)
	key := p.labelOrKeyRule()
	if key == nil {
		p.setTokenizerMode(oldMode)
		return nil
	}
	equals := p.nextToken()
	p.setTokenizerMode(oldMode)

	if equals.Kind != Equals {
		p.ungetToken(equals)
		id := "MissingEqualsInHashLiteral"
		if parsingSchemaElement {
			id = "MissingEqualsInPropertyAssignmentBlock"
		}
		p.reportError(after(key.Extent()), id)
	}

	p.setTokenizerMode(ModeCommand)
	p.skipNewlines()
	statement := p.statementRule()
	if statement == nil {
		id := "MissingStatementInHashLiteral"
		if parsingSchemaElement {
			id = "MissingEqualsInPropertyAssignmentBlock"
		}
		p.reportIncompleteInput(after(equals.Extent), id)
	}
	p.setTokenizerMode(oldMode)
	return &KeyValuePair{Key: key, Value: statement}
}

// scriptBlockExpressionRule is ScriptBlockExpressionRule: { ... } as a value.
func (p *parser) scriptBlockExpressionRule(lCurly *Token) ExpressionAst {
	oldDisableCommaOperator := p.disableCommaOperator
	oldMode := p.tok.Mode
	p.disableCommaOperator = false
	p.setTokenizerMode(ModeCommand)
	p.skipNewlines()
	sb := p.scriptBlockRule(lCurly, false, nil)
	p.disableCommaOperator = oldDisableCommaOperator
	p.setTokenizerMode(oldMode)
	return newScriptBlockExpressionAst(sb.Extent(), sb)
}

// subExpressionRule is SubExpressionRule, for $( and @(.
func (p *parser) subExpressionRule(firstToken *Token) ExpressionAst {
	traps := []*TrapStatementAst{}
	statements := []StatementAst{}

	oldDisableCommaOperator := p.disableCommaOperator
	oldMode := p.tok.Mode
	p.disableCommaOperator = false
	p.setTokenizerMode(ModeCommand)
	p.skipNewlines()
	statementListExtent := p.statementListRule(&statements, &traps)
	p.skipNewlines()
	rParen := p.nextToken()
	if rParen.Kind != RParen {
		p.ungetToken(rParen)
		p.reportIncompleteInput(rParen.Extent, "MissingEndParenthesisInSubexpression")
	}
	p.disableCommaOperator = oldDisableCommaOperator
	p.setTokenizerMode(oldMode)

	// statementListRule returns emptyExtent where C# returns null, which is also what C# substitutes here.
	ext := extentOf(firstToken.Extent, rParen.Extent)
	block := newStatementBlockAst(statementListExtent, statements, traps)
	if firstToken.Kind == DollarParen {
		return newSubExpressionAst(ext, block)
	}
	return newArrayExpressionAst(ext, block)
}

// parenthesizedExpressionRule is ParenthesizedExpressionRule: ( pipeline-chain ).
func (p *parser) parenthesizedExpressionRule(lParen *Token) ExpressionAst {
	oldMode := p.tok.Mode
	oldDisableCommaOperator := p.disableCommaOperator
	p.setTokenizerMode(ModeCommand)
	p.disableCommaOperator = false

	p.skipNewlines()
	pipelineAst := p.pipelineChainRule()
	if pipelineAst == nil {
		p.reportIncompleteInput(after(lParen.Extent), "ExpectedExpression")
	}
	p.skipNewlines()
	rParen := p.nextToken()
	if rParen.Kind != RParen {
		p.ungetToken(rParen)
		p.reportIncompleteInput(after(pipelineAst.Extent()), "MissingEndParenthesisInExpression")
	}
	p.disableCommaOperator = oldDisableCommaOperator
	p.setTokenizerMode(oldMode)
	return newParenExpressionAst(extentOf(lParen.Extent, rParen.Extent), pipelineAst)
}
