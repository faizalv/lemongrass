// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// The expression half of Parser.cs: param blocks and attributes, type names, class, enum and using definitions,
// and expressions. Each method's doc names the C# rule it ports; an out or ref parameter there becomes a result or
// a pointer here. Error recovery branches are left out, since reportError stops the parse.

// paramBlockRule is ParamBlockRule; nil when no param block starts here.
func (p *parser) paramBlockRule() *ParamBlockAst { panic(notPorted) }

// parameterListRule is ParameterListRule.
func (p *parser) parameterListRule() []*ParameterAst { panic(notPorted) }

// parameterRule is ParameterRule.
func (p *parser) parameterRule() *ParameterAst { panic(notPorted) }

// attributeListRule is AttributeListRule.
func (p *parser) attributeListRule(inExpressionMode bool) []AttributeBaseAst { panic(notPorted) }

// attributeRule is AttributeRule.
func (p *parser) attributeRule() AttributeBaseAst { panic(notPorted) }

// attributeArgumentsRule is AttributeArgumentsRule.
func (p *parser) attributeArgumentsRule(positional *[]ExpressionAst, named *[]*NamedAttributeArgumentAst, lastItemExtent *Extent) {
	panic(notPorted)
}

// typeNameRule is TypeNameRule.
func (p *parser) typeNameRule(allowAssemblyQualifiedNames bool) (name ITypeName, firstTypeNameToken *Token) {
	panic(notPorted)
}

// finishTypeNameRule is FinishTypeNameRule.
func (p *parser) finishTypeNameRule(typeName *Token, unBracketedGenericArg, allowAssemblyQualifiedNames bool) ITypeName {
	panic(notPorted)
}

// getSingleGenericArgument is GetSingleGenericArgument.
func (p *parser) getSingleGenericArgument(firstToken *Token) ITypeName { panic(notPorted) }

// genericTypeArgumentsRule is GenericTypeArgumentsRule.
func (p *parser) genericTypeArgumentsRule(firstToken *Token) (args []ITypeName, lastToken *Token) {
	panic(notPorted)
}

// genericTypeNameRule is GenericTypeNameRule.
func (p *parser) genericTypeNameRule(genericTypeName, firstToken *Token, unbracketedGenericArg bool) ITypeName {
	panic(notPorted)
}

// completeArrayTypeName is CompleteArrayTypeName.
func (p *parser) completeArrayTypeName(elementType ITypeName, typeForAssemblyQualification *TypeName, firstTokenAfterLBracket *Token, unBracketedGenericArg bool) ITypeName {
	panic(notPorted)
}

// usingStatementsRule is UsingStatementsRule.
func (p *parser) usingStatementsRule() []*UsingStatementAst { panic(notPorted) }

// usingStatementRule is UsingStatementRule.
func (p *parser) usingStatementRule(usingToken *Token) StatementAst { panic(notPorted) }

// classDefinitionRule is ClassDefinitionRule.
func (p *parser) classDefinitionRule(customAttributes []AttributeBaseAst, classToken *Token) StatementAst {
	panic(notPorted)
}

// enumDefinitionRule is EnumDefinitionRule.
func (p *parser) enumDefinitionRule(customAttributes []AttributeBaseAst, enumToken *Token) StatementAst {
	panic(notPorted)
}

// methodDeclarationRule is MethodDeclarationRule.
func (p *parser) methodDeclarationRule(functionNameToken *Token, className string, isStaticMethod bool) StatementAst {
	panic(notPorted)
}

// expressionRule is ExpressionRule.
func (p *parser) expressionRule(endNumberOnTernaryOpChars bool) ExpressionAst { panic(notPorted) }

// binaryExpressionRule is BinaryExpressionRule.
func (p *parser) binaryExpressionRule(endNumberOnTernaryOpChars bool) ExpressionAst { panic(notPorted) }

// arrayLiteralRule is ArrayLiteralRule.
func (p *parser) arrayLiteralRule(endNumberOnTernaryOpChars bool) ExpressionAst { panic(notPorted) }

// unaryExpressionRule is UnaryExpressionRule.
func (p *parser) unaryExpressionRule(endNumberOnTernaryOpChars bool) ExpressionAst { panic(notPorted) }

// primaryExpressionRule is PrimaryExpressionRule.
func (p *parser) primaryExpressionRule(withMemberAccess bool) ExpressionAst { panic(notPorted) }

// checkUsingVariable is CheckUsingVariable.
func (p *parser) checkUsingVariable(variableToken *Token, withMemberAccess bool) ExpressionAst {
	panic(notPorted)
}

// checkPostPrimaryExpressionOperators is CheckPostPrimaryExpressionOperators.
func (p *parser) checkPostPrimaryExpressionOperators(t *Token, expr ExpressionAst) ExpressionAst {
	panic(notPorted)
}

// hashExpressionRule is HashExpressionRule.
func (p *parser) hashExpressionRule(atCurlyToken *Token, parsingSchemaElement bool) ExpressionAst {
	panic(notPorted)
}

// getKeyValuePair is GetKeyValuePair; nil when no entry starts here.
func (p *parser) getKeyValuePair(parsingSchemaElement bool) *KeyValuePair { panic(notPorted) }

// scriptBlockExpressionRule is ScriptBlockExpressionRule.
func (p *parser) scriptBlockExpressionRule(lCurly *Token) ExpressionAst { panic(notPorted) }

// subExpressionRule is SubExpressionRule, for $( and @(.
func (p *parser) subExpressionRule(firstToken *Token) ExpressionAst { panic(notPorted) }

// parenthesizedExpressionRule is ParenthesizedExpressionRule.
func (p *parser) parenthesizedExpressionRule(lParen *Token) ExpressionAst { panic(notPorted) }

// parseNestedExpressions is ParseNestedExpressions, for the $(...) and $var parts of an expandable string.
func (p *parser) parseNestedExpressions(expandableStringToken *Token) []ExpressionAst {
	panic(notPorted)
}

// expandableStringRule is ExpandableStringRule.
func (p *parser) expandableStringRule(strToken *Token) ExpressionAst { panic(notPorted) }

// memberNameRule is MemberNameRule.
func (p *parser) memberNameRule() ExpressionAst { panic(notPorted) }

// memberAccessRule is MemberAccessRule.
func (p *parser) memberAccessRule(targetExpr ExpressionAst, operatorToken *Token) ExpressionAst {
	panic(notPorted)
}

// genericMethodArgumentsRule is GenericMethodArgumentsRule.
func (p *parser) genericMethodArgumentsRule(resyncIndex int) (args []ITypeName, rBracketToken *Token) {
	panic(notPorted)
}

// memberInvokeRule is MemberInvokeRule.
func (p *parser) memberInvokeRule(targetExpr ExpressionAst, lBracket, operatorToken *Token, member CommandElementAst, genericTypes []ITypeName) ExpressionAst {
	panic(notPorted)
}

// invokeParamParenListRule is InvokeParamParenListRule.
func (p *parser) invokeParamParenListRule(lParen *Token) (args []ExpressionAst, lastExtent Extent) {
	panic(notPorted)
}

// elementAccessRule is ElementAccessRule.
func (p *parser) elementAccessRule(primaryExpression ExpressionAst, lBracket *Token) ExpressionAst {
	panic(notPorted)
}
