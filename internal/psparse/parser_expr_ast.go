// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// Constructors for the nodes the expression half creates, porting the logic of their ast.cs constructors: where C#
// substitutes an empty collection for null the slice is empty and non-nil, and where C# leaves it null it is nil,
// so the tree reads as PowerShell's does. The statement half writes the constructors for its own nodes.

// newVariableExpressionAst is VariableExpressionAst(VariableToken).
func newVariableExpressionAst(t *Token) *VariableExpressionAst {
	return newVariableExpressionAstPath(t.Extent, t.VariablePath, t.Kind == SplattedVariable)
}

// newVariableExpressionAstPath is VariableExpressionAst(extent, variablePath, splatted).
func newVariableExpressionAstPath(ext Extent, path *VariablePath, splatted bool) *VariableExpressionAst {
	n := &VariableExpressionAst{VariablePath: path, Splatted: splatted}
	n.Ext = ext
	return n
}

// stringConstantTypeOf is StringConstantExpressionAst.MapTokenKindToStringConstantKind. C# throws for any other
// kind, which its callers never pass; a bare word stands in so the parser cannot panic.
func stringConstantTypeOf(t *Token) StringConstantType {
	switch t.Kind {
	case StringExpandable:
		return DoubleQuoted
	case HereStringLiteral:
		return SingleQuotedHereString
	case HereStringExpandable:
		return DoubleQuotedHereString
	case StringLiteral:
		return SingleQuoted
	}
	return BareWord
}

// newStringConstantExpressionAst is StringConstantExpressionAst(extent, value, stringConstantType).
func newStringConstantExpressionAst(ext Extent, value string, typ StringConstantType) *StringConstantExpressionAst {
	n := &StringConstantExpressionAst{Value: value, StringConstantType: typ}
	n.Ext = ext
	return n
}

// newStringConstantExpressionAstFromToken is StringConstantExpressionAst(StringToken).
func newStringConstantExpressionAstFromToken(t *Token) *StringConstantExpressionAst {
	return newStringConstantExpressionAst(t.Extent, t.StringValue, stringConstantTypeOf(t))
}

// newConstantExpressionAst is ConstantExpressionAst(extent, value).
func newConstantExpressionAst(ext Extent, value any) *ConstantExpressionAst {
	n := &ConstantExpressionAst{Value: value}
	n.Ext = ext
	return n
}

// newConstantExpressionAstFromNumber is ConstantExpressionAst(NumberToken).
func newConstantExpressionAstFromNumber(t *Token) *ConstantExpressionAst {
	return newConstantExpressionAst(t.Extent, t.Value)
}

// newExpandableStringExpressionAst is ExpandableStringExpressionAst(Token, value, formatString, nested).
func newExpandableStringExpressionAst(t *Token, value, formatString string, nested []ExpressionAst) *ExpandableStringExpressionAst {
	n := &ExpandableStringExpressionAst{
		Value:              value,
		StringConstantType: stringConstantTypeOf(t),
		NestedExpressions:  nested,
		FormatString:       formatString,
	}
	n.Ext = t.Extent
	return n
}

// newArrayLiteralAst is ArrayLiteralAst(extent, elements); elements is never empty.
func newArrayLiteralAst(ext Extent, elements []ExpressionAst) *ArrayLiteralAst {
	n := &ArrayLiteralAst{Elements: elements}
	n.Ext = ext
	return n
}

// newTernaryExpressionAst is TernaryExpressionAst.
func newTernaryExpressionAst(ext Extent, condition, ifTrue, ifFalse ExpressionAst) *TernaryExpressionAst {
	n := &TernaryExpressionAst{Condition: condition, IfTrue: ifTrue, IfFalse: ifFalse}
	n.Ext = ext
	return n
}

// newBinaryExpressionAst is BinaryExpressionAst.
func newBinaryExpressionAst(ext Extent, left ExpressionAst, op TokenKind, right ExpressionAst, errorPosition Extent) *BinaryExpressionAst {
	n := &BinaryExpressionAst{Operator: op, Left: left, Right: right, ErrorPosition: errorPosition}
	n.Ext = ext
	return n
}

// newUnaryExpressionAst is UnaryExpressionAst.
func newUnaryExpressionAst(ext Extent, kind TokenKind, child ExpressionAst) *UnaryExpressionAst {
	n := &UnaryExpressionAst{TokenKind: kind, Child: child}
	n.Ext = ext
	return n
}

// newAttributedExpressionAst is AttributedExpressionAst.
func newAttributedExpressionAst(ext Extent, attribute AttributeBaseAst, child ExpressionAst) *AttributedExpressionAst {
	n := &AttributedExpressionAst{Attribute: attribute, Child: child}
	n.Ext = ext
	return n
}

// newConvertExpressionAst is ConvertExpressionAst; its Attribute and Type are the same constraint.
func newConvertExpressionAst(ext Extent, typeConstraint *TypeConstraintAst, child ExpressionAst) *ConvertExpressionAst {
	n := &ConvertExpressionAst{Child: child, Attribute: typeConstraint, Type: typeConstraint}
	n.Ext = ext
	return n
}

// newTypeExpressionAst is TypeExpressionAst.
func newTypeExpressionAst(ext Extent, typeName ITypeName) *TypeExpressionAst {
	n := &TypeExpressionAst{TypeName: typeName}
	n.Ext = ext
	return n
}

// newMemberExpressionAst is MemberExpressionAst.
func newMemberExpressionAst(ext Extent, expression ExpressionAst, member CommandElementAst, static, nullConditional bool) *MemberExpressionAst {
	n := &MemberExpressionAst{Expression: expression, Member: member, Static: static, NullConditional: nullConditional}
	n.Ext = ext
	return n
}

// newInvokeMemberExpressionAst is InvokeMemberExpressionAst: Arguments and GenericTypeArguments stay nil when
// empty, as C# leaves them null.
func newInvokeMemberExpressionAst(ext Extent, expression ExpressionAst, method CommandElementAst, arguments []ExpressionAst, static, nullConditional bool, genericTypes []ITypeName) *InvokeMemberExpressionAst {
	n := &InvokeMemberExpressionAst{Expression: expression, Member: method, Static: static, NullConditional: nullConditional}
	n.Ext = ext
	if len(arguments) > 0 {
		n.Arguments = arguments
	}
	if len(genericTypes) > 0 {
		n.GenericTypeArguments = genericTypes
	}
	return n
}

// newBaseCtorInvokeMemberExpressionAst is BaseCtorInvokeMemberExpressionAst: a call of $this's .ctor, at the
// base keyword, or at the empty extent for an implicit base constructor call.
func newBaseCtorInvokeMemberExpressionAst(baseKeywordExtent, baseCallExtent Extent, arguments []ExpressionAst) *BaseCtorInvokeMemberExpressionAst {
	this := newVariableExpressionAstPath(baseKeywordExtent, NewVariablePath("this"), false)
	ctor := newStringConstantExpressionAst(baseKeywordExtent, ".ctor", BareWord)
	return &BaseCtorInvokeMemberExpressionAst{
		InvokeMemberExpressionAst: *newInvokeMemberExpressionAst(baseCallExtent, this, ctor, arguments, false, false, nil),
	}
}

// newIndexExpressionAst is IndexExpressionAst.
func newIndexExpressionAst(ext Extent, target, index ExpressionAst, nullConditional bool) *IndexExpressionAst {
	n := &IndexExpressionAst{Target: target, Index: index, NullConditional: nullConditional}
	n.Ext = ext
	return n
}

// newHashtableAst is HashtableAst.
func newHashtableAst(ext Extent, pairs []KeyValuePair) *HashtableAst {
	if pairs == nil {
		pairs = []KeyValuePair{}
	}
	n := &HashtableAst{KeyValuePairs: pairs}
	n.Ext = ext
	return n
}

// newScriptBlockExpressionAst is ScriptBlockExpressionAst.
func newScriptBlockExpressionAst(ext Extent, sb *ScriptBlockAst) *ScriptBlockExpressionAst {
	n := &ScriptBlockExpressionAst{ScriptBlock: sb}
	n.Ext = ext
	return n
}

// newSubExpressionAst is SubExpressionAst.
func newSubExpressionAst(ext Extent, block *StatementBlockAst) *SubExpressionAst {
	n := &SubExpressionAst{SubExpression: block}
	n.Ext = ext
	return n
}

// newArrayExpressionAst is ArrayExpressionAst.
func newArrayExpressionAst(ext Extent, block *StatementBlockAst) *ArrayExpressionAst {
	n := &ArrayExpressionAst{SubExpression: block}
	n.Ext = ext
	return n
}

// newParenExpressionAst is ParenExpressionAst.
func newParenExpressionAst(ext Extent, pipeline PipelineBaseAst) *ParenExpressionAst {
	n := &ParenExpressionAst{Pipeline: pipeline}
	n.Ext = ext
	return n
}

// newUsingExpressionAst is UsingExpressionAst.
func newUsingExpressionAst(ext Extent, child ExpressionAst) *UsingExpressionAst {
	n := &UsingExpressionAst{SubExpression: child}
	n.Ext = ext
	return n
}

// newParamBlockAst is ParamBlockAst.
func newParamBlockAst(ext Extent, attributes []*AttributeAst, parameters []*ParameterAst) *ParamBlockAst {
	if attributes == nil {
		attributes = []*AttributeAst{}
	}
	if parameters == nil {
		parameters = []*ParameterAst{}
	}
	n := &ParamBlockAst{Attributes: attributes, Parameters: parameters}
	n.Ext = ext
	return n
}

// newParameterAst is ParameterAst.
func newParameterAst(ext Extent, name *VariableExpressionAst, attributes []AttributeBaseAst, defaultValue ExpressionAst) *ParameterAst {
	if attributes == nil {
		attributes = []AttributeBaseAst{}
	}
	n := &ParameterAst{Attributes: attributes, Name: name, DefaultValue: defaultValue}
	n.Ext = ext
	return n
}

// newAttributeAst is AttributeAst.
func newAttributeAst(ext Extent, typeName ITypeName, positional []ExpressionAst, named []*NamedAttributeArgumentAst) *AttributeAst {
	if positional == nil {
		positional = []ExpressionAst{}
	}
	if named == nil {
		named = []*NamedAttributeArgumentAst{}
	}
	n := &AttributeAst{TypeName: typeName, PositionalArguments: positional, NamedArguments: named}
	n.Ext = ext
	return n
}

// newNamedAttributeArgumentAst is NamedAttributeArgumentAst.
func newNamedAttributeArgumentAst(ext Extent, name string, argument ExpressionAst, expressionOmitted bool) *NamedAttributeArgumentAst {
	n := &NamedAttributeArgumentAst{ArgumentName: name, Argument: argument, ExpressionOmitted: expressionOmitted}
	n.Ext = ext
	return n
}

// newTypeConstraintAst is TypeConstraintAst.
func newTypeConstraintAst(ext Extent, typeName ITypeName) *TypeConstraintAst {
	n := &TypeConstraintAst{TypeName: typeName}
	n.Ext = ext
	return n
}

// newTypeDefinitionAst is TypeDefinitionAst: attributes, members and base types are empty, never nil.
func newTypeDefinitionAst(ext Extent, name string, attributes []*AttributeAst, members []MemberAst, typeAttributes TypeAttributes, baseTypes []*TypeConstraintAst) *TypeDefinitionAst {
	if len(attributes) == 0 {
		attributes = []*AttributeAst{}
	}
	if len(members) == 0 {
		members = []MemberAst{}
	}
	if len(baseTypes) == 0 {
		baseTypes = []*TypeConstraintAst{}
	}
	n := &TypeDefinitionAst{Name: name, Attributes: attributes, BaseTypes: baseTypes, Members: members, TypeAttributes: typeAttributes}
	n.Ext = ext
	return n
}

// newPropertyMemberAst is PropertyMemberAst.
func newPropertyMemberAst(ext Extent, name string, propertyType *TypeConstraintAst, attributes []*AttributeAst, propertyAttributes PropertyAttributes, initialValue ExpressionAst) *PropertyMemberAst {
	if attributes == nil {
		attributes = []*AttributeAst{}
	}
	n := &PropertyMemberAst{Name: name, PropertyType: propertyType, Attributes: attributes, PropertyAttributes: propertyAttributes, InitialValue: initialValue}
	n.Ext = ext
	return n
}

// newFunctionMemberAst is FunctionMemberAst: Name, Parameters and Body are read from the wrapped function, with
// Parameters empty rather than nil, as the C# properties read them.
func newFunctionMemberAst(ext Extent, fn *FunctionDefinitionAst, returnType *TypeConstraintAst, attributes []*AttributeAst, methodAttributes MethodAttributes) *FunctionMemberAst {
	if attributes == nil {
		attributes = []*AttributeAst{}
	}
	params := fn.Parameters
	if params == nil {
		params = []*ParameterAst{}
	}
	n := &FunctionMemberAst{
		Name:             fn.Name,
		Attributes:       attributes,
		ReturnType:       returnType,
		Parameters:       params,
		Body:             fn.Body,
		MethodAttributes: methodAttributes,
	}
	n.Ext = ext
	return n
}

// newUsingStatementAst is UsingStatementAst: name is the item, alias the aliased value when an '=' follows, and
// moduleSpecification the hashtable of using module.
func newUsingStatementAst(ext Extent, kind UsingStatementKind, name, alias *StringConstantExpressionAst, moduleSpecification *HashtableAst) *UsingStatementAst {
	n := &UsingStatementAst{UsingStatementKind: kind, Name: name, Alias: alias, ModuleSpecification: moduleSpecification}
	n.Ext = ext
	return n
}
