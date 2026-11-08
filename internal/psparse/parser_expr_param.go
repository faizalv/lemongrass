// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// Param blocks, parameters and attributes. An attribute and a type constraint both start with '['; the token after
// the type name tells them apart: '(' makes an attribute with arguments, ']' a type constraint.

// paramBlockRule is ParamBlockRule; nil when no param block starts here. Without a '(' after param this is not an
// error: the word is read as a command named param.
func (p *parser) paramBlockRule() *ParamBlockAst {
	p.skipNewlines()
	candidateAttributes := p.attributeListRule(false)
	p.skipNewlines()
	paramToken := p.peekToken()
	if paramToken.Kind != Param {
		return nil
	}
	p.skipToken()
	p.skipNewlines()
	lParen := p.nextToken()
	if lParen.Kind != LParen {
		p.ungetToken(lParen)
		return nil
	}

	parameters := p.parameterListRule()
	p.skipNewlines()
	rParen := p.nextToken()
	if rParen.Kind != RParen {
		p.ungetToken(rParen)
		pos := after(lParen.Extent)
		if len(parameters) > 0 {
			pos = after(parameters[len(parameters)-1].Extent())
		}
		p.reportIncompleteInput(pos, "MissingEndParenthesisInFunctionParameterList")
	}

	attributes := []*AttributeAst{}
	for _, attr := range candidateAttributes {
		a, ok := attr.(*AttributeAst)
		if !ok {
			// Only attributes may precede param, never a type constraint.
			p.reportError(attr.Extent(), "TypeNotAllowedBeforeParam", attr.GetTypeName().FullName())
		}
		attributes = append(attributes, a)
	}
	return newParamBlockAst(extentOf(paramToken.Extent, rParen.Extent), attributes, parameters)
}

// parameterListRule is ParameterListRule: parameters separated by commas.
func (p *parser) parameterListRule() []*ParameterAst {
	parameters := []*ParameterAst{}
	var commaToken *Token
	for {
		parameter := p.parameterRule()
		if parameter == nil {
			if commaToken != nil {
				p.reportIncompleteInput(after(commaToken.Extent), "MissingExpressionAfterToken", commaToken.Kind.Text())
			}
			break
		}
		parameters = append(parameters, parameter)
		p.skipNewlines()
		commaToken = p.peekToken()
		if commaToken.Kind != Comma {
			break
		}
		p.skipToken()
	}
	return parameters
}

// parameterRule is ParameterRule: attributes, a variable and an optional default value; nil when no variable
// starts here. It reads in expression mode with the comma operator off, since a comma separates parameters.
func (p *parser) parameterRule() *ParameterAst {
	oldDisableCommaOperator := p.disableCommaOperator
	oldMode := p.tok.Mode
	p.disableCommaOperator = true
	p.setTokenizerMode(ModeExpression)

	p.skipNewlines()
	attributes := p.attributeListRule(false)
	p.skipNewlines()
	t := p.nextToken()
	if t.Kind != Variable && t.Kind != SplattedVariable {
		p.ungetToken(t)
		if attributes != nil {
			p.reportIncompleteInput(after(attributes[len(attributes)-1].Extent()), "InvalidFunctionParameter")
		}
		p.disableCommaOperator = oldDisableCommaOperator
		p.setTokenizerMode(oldMode)
		return nil
	}
	variableToken := t

	var defaultValue ExpressionAst
	p.skipNewlines()
	equalsToken := p.peekToken()
	if equalsToken.Kind == Equals {
		p.skipToken()
		p.skipNewlines()
		defaultValue = p.expressionRule(false)
		if defaultValue == nil {
			p.reportIncompleteInput(after(equalsToken.Extent), "MissingExpressionAfterToken", equalsToken.Kind.Text())
		}
	}
	p.disableCommaOperator = oldDisableCommaOperator
	p.setTokenizerMode(oldMode)

	start := variableToken.Extent
	if attributes != nil {
		start = attributes[0].Extent()
	}
	end := variableToken.Extent
	if defaultValue != nil {
		end = defaultValue.Extent()
	}
	return newParameterAst(extentOf(start, end), newVariableExpressionAst(variableToken), attributes, defaultValue)
}

// attributeListRule is AttributeListRule; nil when no attribute starts here. In expression mode a newline after a
// type constraint ends the list, since [int]<newline>$x is a type expression followed by a new statement.
func (p *parser) attributeListRule(inExpressionMode bool) []AttributeBaseAst {
	var attributes []AttributeBaseAst
	attribute := p.attributeRule()
	for attribute != nil {
		attributes = append(attributes, attribute)
		if _, isAttribute := attribute.(*AttributeAst); !inExpressionMode || isAttribute {
			p.skipNewlines()
		}
		attribute = p.attributeRule()
	}
	if len(attributes) == 0 {
		return nil
	}
	return attributes
}

// attributeRule is AttributeRule: [Name(args)] or a type constraint [Name]; nil when no '[' starts here.
func (p *parser) attributeRule() AttributeBaseAst {
	lBracket := p.nextLBracket()
	if lBracket == nil {
		return nil
	}
	p.skipNewlines()

	typeName, firstTypeNameToken := p.typeNameRule(true)
	if typeName == nil {
		// typeNameRule may have read tokens past the bracket.
		p.resyncToken(lBracket)
		p.reportIncompleteInput(after(lBracket.Extent), "MissingTypename")
	}

	lParenOrRBracket := p.nextToken()
	if lParenOrRBracket.Kind == LParen {
		p.skipNewlines()
		positionalArguments := []ExpressionAst{}
		namedArguments := []*NamedAttributeArgumentAst{}
		lastItemExtent := lParenOrRBracket.Extent

		oldMode := p.tok.Mode
		p.setTokenizerMode(ModeExpression)
		p.attributeArgumentsRule(&positionalArguments, &namedArguments, &lastItemExtent)
		p.setTokenizerMode(oldMode)

		p.skipNewlines()
		rParen := p.nextToken()
		if rParen.Kind != RParen {
			p.ungetToken(rParen)
			p.reportIncompleteInput(after(lastItemExtent), "MissingEndParenthesisInExpression")
		}
		p.skipNewlines()
		rBracket := p.nextToken()
		if rBracket.Kind != RBracket {
			p.ungetToken(rBracket)
			p.reportIncompleteInput(after(rParen.Extent), "EndSquareBracketExpectedAtEndOfAttribute")
		}
		firstTypeNameToken.Flags |= FlagAttributeName
		return newAttributeAst(extentOf(lBracket.Extent, rBracket.Extent), typeName, positionalArguments, namedArguments)
	}

	if lParenOrRBracket.Kind != RBracket {
		p.ungetToken(lParenOrRBracket)
		p.reportError(before(lParenOrRBracket.Extent), "EndSquareBracketExpectedAtEndOfAttribute")
	}
	return newTypeConstraintAst(extentOf(lBracket.Extent, lParenOrRBracket.Extent), typeName)
}

// attributeArgumentsRule is AttributeArgumentsRule: positional expressions and Name = value pairs separated by
// commas; a Name alone means Name = $true.
//
// C# declares a set of names seen for a DuplicateNamedArgument error but never adds to it, so a repeated name is
// not an error in PowerShell; it is not checked here either.
func (p *parser) attributeArgumentsRule(positionalArguments *[]ExpressionAst, namedArguments *[]*NamedAttributeArgumentAst, lastItemExtent *Extent) {
	oldDisableCommaOperator := p.disableCommaOperator
	p.disableCommaOperator = true
	var commaToken *Token
	for {
		p.skipNewlines()
		name, _ := p.simpleNameRule()
		var expr ExpressionAst
		expressionOmitted := false
		if name != nil {
			t := p.peekToken()
			if t.Kind == Equals {
				t = p.nextToken()
				p.skipNewlines()
				expr = p.expressionRule(false)
				if expr == nil {
					p.reportIncompleteInput(after(t.Extent), "MissingExpressionInNamedArgument")
				}
				*lastItemExtent = expr.Extent()
			} else {
				expr = newConstantExpressionAst(name.Extent(), true)
				expressionOmitted = true
			}
		} else {
			expr = p.expressionRule(false)
		}

		switch {
		case name != nil:
			*namedArguments = append(*namedArguments,
				newNamedAttributeArgumentAst(extentOf(name.Extent(), expr.Extent()), name.Value, expr, expressionOmitted))
		case expr != nil:
			*positionalArguments = append(*positionalArguments, expr)
			*lastItemExtent = expr.Extent()
		case commaToken != nil:
			p.reportIncompleteInput(after(commaToken.Extent), "MissingExpressionAfterToken", commaToken.Kind.Text())
		}

		p.skipNewlines()
		commaToken = p.peekToken()
		if commaToken.Kind != Comma {
			break
		}
		*lastItemExtent = commaToken.Extent
		p.skipToken()
	}
	p.disableCommaOperator = oldDisableCommaOperator
}
