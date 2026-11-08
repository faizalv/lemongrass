// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// Member access, method invocation and indexing after a primary expression: .x, ::x, ?.x, .m(...), .m[T](...),
// .m{...}, [i] and ?[i]. The tokenizer hands out the access operators only when nothing separates them from the
// expression before, which is what makes $a.b a member access and $a .b two arguments.

// checkPostPrimaryExpressionOperators is CheckPostPrimaryExpressionOperators: it applies member and element
// accesses for as long as the tokenizer finds another directly after the expression.
func (p *parser) checkPostPrimaryExpressionOperators(t *Token, expr ExpressionAst) ExpressionAst {
	for t != nil {
		// Fluent style: a newline may follow the operator.
		p.skipNewlines()
		switch t.Kind {
		case Dot, ColonColon, QuestionDot:
			expr = p.memberAccessRule(expr, t)
		case LBracket, QuestionLBracket:
			expr = p.elementAccessRule(expr, t)
		}
		t = p.nextMemberAccessToken(true)
	}
	return expr
}

// memberNameRule is MemberNameRule: a simple name, or any value or unary expression ($a.$name, $a.'x', $a.(1)).
func (p *parser) memberNameRule() ExpressionAst {
	if simpleName, _ := p.simpleNameRule(); simpleName != nil {
		return simpleName
	}
	t := p.peekToken()
	if t.Kind.HasTrait(FlagUnaryOperator) || t.Kind == LBracket {
		return p.unaryExpressionRule(false)
	}
	return p.primaryExpressionRule(false)
}

// memberAccessRule is MemberAccessRule: a property access, or a method call when generic arguments or an opening
// parenthesis or brace follow the name directly.
func (p *parser) memberAccessRule(targetExpr ExpressionAst, operatorToken *Token) ExpressionAst {
	member := p.memberNameRule()
	if member == nil {
		p.reportIncompleteInput(after(operatorToken.Extent), "MissingPropertyName")
	}
	// A pushed-back token means the member name was incomplete, as in $a.$(Get-X; then no call is looked for.
	if p.ungotToken == nil {
		resyncIndex := p.tok.getRestorePoint()
		genericTypeArguments, rBracket := p.genericMethodArgumentsRule(resyncIndex)
		lParen := p.nextInvokeMemberToken()
		if lParen != nil {
			return p.memberInvokeRule(targetExpr, lParen, operatorToken, member, genericTypeArguments)
		}
		if rBracket != nil {
			// Generic arguments with no call after them were an index into the property instead: rescan them.
			p.resync(resyncIndex)
		}
	}
	return newMemberExpressionAst(extentOf(targetExpr.Extent(), member.Extent()), targetExpr, member,
		operatorToken.Kind == ColonColon, operatorToken.Kind == QuestionDot)
}

// genericMethodArgumentsRule is GenericMethodArgumentsRule: [T1, T2] after a method name. Only unbracketed type
// names are taken, since $a.Prop[[T]::X] would be ambiguous with indexing. When no generic arguments are found
// the tokenizer is rewound to resyncIndex.
func (p *parser) genericMethodArgumentsRule(resyncIndex int) (args []ITypeName, rBracketToken *Token) {
	lBracket := p.nextToken()
	if lBracket.Kind != LBracket {
		// A rewind rather than a pushed-back token: the member and invoke helpers after this treat a pushed-back
		// token as an error state.
		p.resync(resyncIndex)
		return nil, nil
	}

	oldMode := p.tok.Mode
	p.setTokenizerMode(ModeTypeName)
	p.skipNewlines()
	firstToken := p.nextToken()
	if firstToken.Kind == Identifier {
		resyncIndex = -1
		args, rBracketToken = p.genericTypeArgumentsRule(firstToken)
		if rBracketToken.Kind != RBracket {
			p.ungetToken(rBracketToken)
			p.reportIncompleteInput(before(rBracketToken.Extent), "EndSquareBracketExpectedAtEndOfType")
		}
	}
	p.setTokenizerMode(oldMode)
	// C# tests > 0, not >= 0; a restore point of 0 cannot follow a member name, so the two never differ.
	if resyncIndex > 0 {
		p.resync(resyncIndex)
	}
	return args, rBracketToken
}

// memberInvokeRule is MemberInvokeRule: lBracket is '(' for an argument list, or '{' for $x.m{...}, read as
// $x.m({...}).
func (p *parser) memberInvokeRule(targetExpr ExpressionAst, lBracket, operatorToken *Token, member CommandElementAst, genericTypes []ITypeName) ExpressionAst {
	var arguments []ExpressionAst
	var lastExtent Extent
	if lBracket.Kind == LParen {
		arguments, lastExtent = p.invokeParamParenListRule(lBracket)
	} else {
		p.skipNewlines()
		argument := p.scriptBlockExpressionRule(lBracket)
		arguments = []ExpressionAst{argument}
		lastExtent = argument.Extent()
	}
	return newInvokeMemberExpressionAst(extentOf(targetExpr.Extent(), lastExtent), targetExpr, member, arguments,
		operatorToken.Kind == ColonColon, operatorToken.Kind == QuestionDot, genericTypes)
}

// invokeParamParenListRule is InvokeParamParenListRule: the arguments after '(' up to ')'. A comma separates
// arguments here, so it is not the array operator.
func (p *parser) invokeParamParenListRule(lParen *Token) (args []ExpressionAst, lastExtent Extent) {
	args = []ExpressionAst{}
	var comma *Token
	oldDisableCommaOperator := p.disableCommaOperator
	p.disableCommaOperator = true
	for {
		p.skipNewlines()
		argument := p.expressionRule(false)
		if argument == nil {
			if comma != nil {
				p.reportIncompleteInput(after(comma.Extent), "MissingExpressionAfterToken", Comma.Text())
			}
			break
		}
		args = append(args, argument)
		p.skipNewlines()
		comma = p.nextToken()
		if comma.Kind != Comma {
			p.ungetToken(comma)
			comma = nil
			break
		}
	}
	p.skipNewlines()
	rParen := p.nextToken()
	if rParen.Kind != RParen {
		p.ungetToken(rParen)
		pos := after(lParen.Extent)
		if len(args) > 0 {
			pos = after(args[len(args)-1].Extent())
		}
		p.reportIncompleteInput(pos, "MissingEndParenthesisInMethodCall")
	}
	p.disableCommaOperator = oldDisableCommaOperator
	return args, rParen.Extent
}

// elementAccessRule is ElementAccessRule: [index] or ?[index] after an expression.
func (p *parser) elementAccessRule(primaryExpression ExpressionAst, lBracket *Token) ExpressionAst {
	p.skipNewlines()
	oldDisableCommaOperator := p.disableCommaOperator
	p.disableCommaOperator = false
	indexExpr := p.expressionRule(false)
	p.disableCommaOperator = oldDisableCommaOperator
	if indexExpr == nil {
		p.reportIncompleteInput(after(lBracket.Extent), "MissingArrayIndexExpression")
	}
	p.skipNewlines()
	rBracket := p.nextToken()
	if rBracket.Kind != RBracket {
		p.ungetToken(rBracket)
		p.reportIncompleteInput(after(indexExpr.Extent()), "MissingEndSquareBracket")
	}
	return newIndexExpressionAst(extentOf(primaryExpression.Extent(), rBracket.Extent), primaryExpression, indexExpr,
		lBracket.Kind == QuestionLBracket)
}
