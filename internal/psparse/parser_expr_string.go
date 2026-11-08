// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// Expandable strings and here-strings. The tokenizer leaves each $(...) inside one unscanned; the parser scans it
// here with a nested tokenizer run and parses it as a primary expression, so "a $($x.y) b" holds a real tree.

// parseNestedExpressions is ParseNestedExpressions: one expression per $var or $(...) part, in order. The
// string token's NestedTokens is replaced by the tokens the nested scans produced, as PowerShell does whenever it
// saves tokens, which Parse always does.
func (p *parser) parseNestedExpressions(expandableStringToken *Token) []ExpressionAst {
	p.enter()
	defer p.leave()

	nestedExpressions := []ExpressionAst{}
	newNestedTokens := []*Token{}
	for _, t := range expandableStringToken.NestedTokens {
		var exprAst ExpressionAst
		if t.Kind == Variable || t.Kind == SplattedVariable {
			exprAst = p.checkUsingVariable(t, false)
			newNestedTokens = append(newNestedTokens, t)
		} else {
			// An unscanned subexpression: StringLiteral kind, its text in StringValue.
			ts := p.tok.startNestedScan(t)
			exprAst = p.primaryExpressionRule(true)
			if p.tok.tokenList != nil {
				newNestedTokens = append(newNestedTokens, *p.tok.tokenList...)
			}
			// The pushed-back token is the nested scan's end of input; it does not belong to the outer scan.
			p.ungotToken = nil
			p.tok.finishNestedScan(ts)
		}
		nestedExpressions = append(nestedExpressions, exprAst)
	}
	expandableStringToken.NestedTokens = newNestedTokens
	return nestedExpressions
}

// expandableStringRule is ExpandableStringRule: a double-quoted string or here-string becomes an expandable
// string only when it holds a variable or subexpression, otherwise a plain string constant.
func (p *parser) expandableStringRule(strToken *Token) ExpressionAst {
	if strToken.NestedTokens != nil {
		nested := p.parseNestedExpressions(strToken)
		return newExpandableStringExpressionAst(strToken, strToken.StringValue, strToken.FormatString, nested)
	}
	return newStringConstantExpressionAstFromToken(strToken)
}
