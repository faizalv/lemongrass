// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import "strings"

// Class, enum and using definitions with their members. Left out on purpose: the ConstrainedLanguage refusal of
// ClassDefinitionRule (this parser reads FullLanguage only) and NextTypeIdentifierToken, which nothing upstream
// calls.

// usingStatementsRule is UsingStatementsRule: the using statements at the start of a script; nil when there are
// none.
func (p *parser) usingStatementsRule() []*UsingStatementAst {
	var result []*UsingStatementAst
	for {
		t := p.peekToken()
		if t.Kind != Using {
			// The caller may scan well past one token speculatively, so its restore point must include this one.
			p.resyncToken(t)
			return result
		}
		p.skipToken()
		statement := p.usingStatementRule(t)
		p.skipNewlinesAndSemicolons()
		if result == nil {
			result = []*UsingStatementAst{}
		}
		if u, ok := statement.(*UsingStatementAst); ok {
			result = append(result, u)
		}
	}
}

// usingStatementRule is UsingStatementRule: using namespace, module, type, command or assembly, with an optional
// alias ('using namespace A = B'), required for type and command.
func (p *parser) usingStatementRule(usingToken *Token) StatementAst {
	directiveToken := p.nextToken()
	var kind UsingStatementKind
	aliasAllowed, aliasRequired := false, false
	switch directiveToken.Kind {
	case Namespace:
		kind, aliasAllowed = UsingNamespace, true
	case Type:
		kind, aliasRequired = UsingType, true
	case Module:
		kind, aliasAllowed = UsingModule, true
	case Command:
		kind, aliasRequired = UsingCommand, true
	case Assembly:
		kind = UsingAssembly
	default:
		p.ungetToken(directiveToken)
		p.reportIncompleteInput(after(usingToken.Extent), "MissingUsingStatementDirective")
	}

	itemToken := p.nextToken()
	switch itemToken.Kind {
	// A comma is refused too: getCommandArgument would read ',name' as a one-element array.
	case EndOfInput, NewLine, Comma, Semi:
		p.reportIncompleteInput(after(directiveToken.Extent), "MissingUsingItemName")
	}

	itemAst := p.getCommandArgument(argCommandArgument, itemToken)
	if itemAst == nil {
		p.reportError(itemToken.Extent, "InvalidValueForUsingItemName", itemToken.Text)
	}
	_, itemIsString := itemAst.(*StringConstantExpressionAst)
	_, itemIsHashtable := itemAst.(*HashtableAst)
	if !itemIsString && (kind != UsingModule || !itemIsHashtable) {
		p.reportError(itemAst.Extent(), "InvalidValueForUsingItemName", p.text(itemAst.Extent()))
	}

	var htAst *HashtableAst
	switch kind {
	case UsingAssembly:
		itemAst = p.resolveUsingAssembly(itemAst.(*StringConstantExpressionAst))
	case UsingModule:
		htAst, _ = itemAst.(*HashtableAst)
	}

	// A module given as a hashtable takes no alias.
	if (aliasAllowed || aliasRequired) && htAst == nil {
		equalsToken := p.peekToken()
		if equalsToken.Kind == Equals {
			p.skipToken()
			aliasToken := p.nextToken()
			switch aliasToken.Kind {
			case EndOfInput, NewLine, Semi:
				p.ungetToken(aliasToken)
				p.reportIncompleteInput(after(equalsToken.Extent), "MissingNamespaceAlias")
			case Comma:
				p.reportError(aliasToken.Extent, "UnexpectedUnaryOperator", aliasToken.Text)
			}

			aliasAst := p.getCommandArgument(argCommandArgument, aliasToken)
			aliasHashtable, aliasIsHashtable := aliasAst.(*HashtableAst)
			aliasString, aliasIsString := aliasAst.(*StringConstantExpressionAst)
			if kind == UsingModule && aliasIsHashtable {
				htAst = aliasHashtable
			} else if !aliasIsString {
				errorExtent := aliasToken.Extent
				if aliasAst != nil {
					errorExtent = aliasAst.Extent()
				}
				p.reportError(errorExtent, "InvalidValueForUsingItemName", p.text(errorExtent))
			}

			p.requireStatementTerminator()
			ext := extentOf(usingToken.Extent, aliasToken.Extent)
			if htAst == nil {
				return newUsingStatementAst(ext, kind, itemAst.(*StringConstantExpressionAst), aliasString, nil)
			}
			return newUsingStatementAst(ext, UsingModule, itemAst.(*StringConstantExpressionAst), nil, htAst)
		}
		if aliasRequired {
			p.reportIncompleteInput(after(itemToken.Extent), "MissingEqualsInUsingAlias")
		}
	}

	p.requireStatementTerminator()
	ext := extentOf(usingToken.Extent, itemAst.Extent())
	if htAst == nil {
		return newUsingStatementAst(ext, kind, itemAst.(*StringConstantExpressionAst), nil, nil)
	}
	return newUsingStatementAst(ext, UsingModule, nil, nil, htAst)
}

// resolveUsingAssembly is ResolveUsingAssembly, with one deliberate divergence. An absolute name is checked as
// .NET's Uri does: a UNC path or a scheme other than file is an error, anything else is kept. PowerShell resolves
// a relative name against the working directory or the script's folder and reports ErrorLoadingAssembly unless
// that file exists; this parser never touches the disk, so a relative name is always ErrorLoadingAssembly.
func (p *parser) resolveUsingAssembly(name *StringConstantExpressionAst) *StringConstantExpressionAst {
	assemblyName := name.Value
	scheme, unc, absolute := absoluteURI(assemblyName)
	if !absolute {
		p.reportError(name.Extent(), "ErrorLoadingAssembly", assemblyName)
	}
	if unc {
		p.reportError(name.Extent(), "CannotLoadAssemblyFromUncPath", assemblyName)
	}
	if scheme != "file" {
		p.reportError(name.Extent(), "CannotLoadAssemblyWithUriSchema", scheme)
	}
	return name
}

// absoluteURI approximates Uri.TryCreate(s, UriKind.Absolute): a drive path (C:\x, C:/x), a UNC path (\\host\x,
// //host/x), a rooted Unix path (/x, absolute on .NET for Unix) and file://host/x are file URIs; scheme:rest with
// a scheme of two or more characters is a URI of that scheme. The gate does not know which OS runs the call, so
// both the Windows and the Unix readings count as absolute.
func absoluteURI(s string) (scheme string, unc, absolute bool) {
	switch {
	case len(s) >= 3 && isASCIILetter(s[0]) && s[1] == ':' && (s[2] == '\\' || s[2] == '/'):
		return "file", false, true
	case strings.HasPrefix(s, `\\`) || strings.HasPrefix(s, "//"):
		return "file", true, true
	case strings.HasPrefix(s, "/") || strings.HasPrefix(s, `\`):
		return "file", false, strings.HasPrefix(s, "/")
	}
	colon := strings.IndexByte(s, ':')
	if colon < 2 || !isASCIILetter(s[0]) {
		return "", false, false
	}
	for i := 1; i < colon; i++ {
		c := s[i]
		if !isASCIILetter(c) && !(c >= '0' && c <= '9') && c != '+' && c != '-' && c != '.' {
			return "", false, false
		}
	}
	scheme = strings.ToLower(s[:colon])
	if scheme == "file" {
		rest := strings.TrimPrefix(s[colon+1:], "//")
		unc = strings.HasPrefix(s[colon+1:], "//") && rest != "" && rest[0] != '/'
	}
	return scheme, unc, true
}

func isASCIILetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

// classDefinitionRule is ClassDefinitionRule: class Name [: Base, Interface] { members }.
func (p *parser) classDefinitionRule(customAttributes []AttributeBaseAst, classToken *Token) StatementAst {
	p.skipNewlines()
	name, classNameToken := p.simpleNameRule()
	if name == nil {
		p.reportIncompleteInput(after(classToken.Extent), "MissingNameAfterKeyword", classToken.Text)
	}
	// The class name names a type, not a member.
	classNameToken.Flags &^= FlagMemberName
	classNameToken.Flags |= FlagTypeName
	p.skipNewlines()

	oldMode := p.tok.Mode
	p.setTokenizerMode(ModeSignature)
	superClassesList := []*TypeConstraintAst{}
	colonToken := p.peekToken()
	if colonToken.Kind == Colon {
		p.skipToken()
		p.skipNewlines()
		var commaToken *Token
		for {
			superClass, _ := p.typeNameRule(false)
			if superClass == nil {
				pos := colonToken.Extent
				if commaToken != nil {
					pos = commaToken.Extent
				}
				p.reportIncompleteInput(after(pos), "TypeNameExpected")
			}
			superClassesList = append(superClassesList, newTypeConstraintAst(superClass.Extent(), superClass))
			p.skipNewlines()
			commaToken = p.peekToken()
			if commaToken.Kind != Comma {
				break
			}
			p.skipToken()
			p.skipNewlines()
		}
	}

	lCurly := p.nextToken()
	if lCurly.Kind != LCurly {
		p.ungetToken(lCurly)
		lastElement := name.Extent()
		if len(superClassesList) > 0 {
			lastElement = superClassesList[len(superClassesList)-1].Extent()
		}
		p.reportIncompleteInput(after(lastElement), "MissingTypeBody", classToken.Kind.Text())
	}

	members := []MemberAst{}
	for {
		// A member rule that returns nothing without an error has reached the closing brace; its error paths
		// stop the parse.
		member := p.classMemberRule(name.Value)
		if member == nil {
			break
		}
		members = append(members, member)
	}

	rCurly := p.nextToken()
	if rCurly.Kind != RCurly {
		p.ungetToken(rCurly)
		p.reportIncompleteInputAt(after(lCurly.Extent), rCurly.Extent, "MissingEndCurlyBrace")
	}

	startExtent := classToken.Extent
	if len(customAttributes) > 0 {
		startExtent = customAttributes[0].Extent()
	}
	ext := extentOf(startExtent, rCurly.Extent)
	attributes, typeConstraint := splitAttributes(customAttributes)
	if typeConstraint {
		// statementRule reports a type constraint before a keyword statement, so this is never reached.
		p.reportError(ext, ErrUnreachable)
	}
	def := newTypeDefinitionAst(ext, name.Value, attributes, members, TypeAttributesClass, superClassesList)
	p.setTokenizerMode(oldMode)
	return def
}

// splitAttributes is customAttributes.OfType<AttributeAst>(), and whether any TypeConstraintAst was among them.
func splitAttributes(customAttributes []AttributeBaseAst) (attributes []*AttributeAst, typeConstraint bool) {
	for _, a := range customAttributes {
		if attr, ok := a.(*AttributeAst); ok {
			attributes = append(attributes, attr)
		} else {
			typeConstraint = true
		}
	}
	return attributes, typeConstraint
}

// classMemberRule is ClassMemberRule: a property ($x, [type] $x = value) or a method (Name(params) { body }),
// after attributes and the static and hidden modifiers; nil when no member starts here. Public and private are
// compiled out upstream (SUPPORT_PUBLIC_PRIVATE), so every member is public.
func (p *parser) classMemberRule(className string) MemberAst {
	var startExtent *Extent
	attributeList := []*AttributeAst{}
	var typeConstraint *TypeConstraintAst
	var staticToken, hiddenToken, t *Token
	var lastAttribute *Extent

	for scanning := true; scanning; {
		p.skipNewlines()
		if attribute := p.attributeRule(); attribute != nil {
			ext := attribute.Extent()
			lastAttribute = &ext
			if startExtent == nil {
				startExtent = &ext
			}
			if a, ok := attribute.(*AttributeAst); ok {
				attributeList = append(attributeList, a)
			} else if typeConstraint == nil {
				typeConstraint = attribute.(*TypeConstraintAst)
			} else {
				p.reportError(attribute.Extent(), "TooManyTypes")
			}
			continue
		}

		t = p.peekToken()
		if startExtent == nil {
			ext := t.Extent
			startExtent = &ext
		}
		switch t.Kind {
		case Hidden:
			if hiddenToken != nil {
				p.reportError(t.Extent, "DuplicateQualifier", t.Text)
			}
			hiddenToken = t
			lastAttribute = &t.Extent
			p.skipToken()
		case Static:
			if staticToken != nil {
				p.reportError(t.Extent, "DuplicateQualifier", t.Text)
			}
			staticToken = t
			lastAttribute = &t.Extent
			p.skipToken()
		default:
			scanning = false
		}
	}

	if t.Kind == Variable {
		p.skipToken()
		var initialValueAst ExpressionAst
		if assignToken := p.peekToken(); assignToken.Kind == Equals {
			p.skipToken()
			p.skipNewlines()
			initialValueAst = p.expressionRule(false)
		}

		attributes := PropertyAttributesPublic
		if staticToken != nil {
			attributes |= PropertyAttributesStatic
		}
		if hiddenToken != nil {
			attributes |= PropertyAttributesHidden
		}

		endExtent := t.Extent
		if initialValueAst != nil {
			endExtent = initialValueAst.Extent()
		}
		terminatorToken := p.peekToken()
		if terminatorToken.Kind != NewLine && terminatorToken.Kind != Semi && terminatorToken.Kind != RCurly {
			p.reportIncompleteInput(after(endExtent), "MissingPropertyTerminator")
		}
		p.skipNewlinesAndSemicolons()
		// The extent takes in a semicolon, but not a newline or closing brace.
		if terminatorToken.Kind == Semi {
			endExtent = terminatorToken.Extent
		}

		propertyName := t.VariablePath.UnqualifiedPath()
		if propertyName == "" {
			// class C { $private: }: the tokenizer has already reported the empty name.
			p.reportError(t.Extent, ErrUnreachable)
		}
		return newPropertyMemberAst(extentOf(*startExtent, endExtent), propertyName, typeConstraint, attributeList, attributes, initialValueAst)
	}

	if tryUseTokenAsSimpleName(t) {
		p.skipToken()
		fn, ok := p.methodDeclarationRule(t, className, staticToken != nil).(*FunctionDefinitionAst)
		if !ok {
			// methodDeclarationRule reports an error on every path that builds no function.
			p.reportError(t.Extent, ErrUnreachable)
		}
		attributes := MethodAttributesPublic
		if staticToken != nil {
			attributes |= MethodAttributesStatic
		}
		if hiddenToken != nil {
			attributes |= MethodAttributesHidden
		}
		return newFunctionMemberAst(extentOf(*startExtent, fn.Extent()), fn, typeConstraint, attributeList, attributes)
	}

	if lastAttribute != nil {
		// The start of a member, but no variable or method name after it.
		p.reportIncompleteInput(after(*lastAttribute), "IncompleteMemberDefinition")
	}
	return nil
}

// tryUseTokenAsSimpleName is TryUseTokenAsSimpleName: an identifier or a keyword may name a method; it then loses
// its keyword flags.
func tryUseTokenAsSimpleName(t *Token) bool {
	if t.Kind == Identifier || t.Kind == DynamicKeyword || t.Flags&FlagKeyword != 0 {
		t.Flags = FlagNone
		return true
	}
	return false
}

// enumDefinitionRule is EnumDefinitionRule: enum Name [: IntegralType] { members }.
func (p *parser) enumDefinitionRule(customAttributes []AttributeBaseAst, enumToken *Token) StatementAst {
	p.skipNewlines()
	name, _ := p.simpleNameRule()
	if name == nil {
		p.reportIncompleteInput(after(enumToken.Extent), "MissingNameAfterKeyword", enumToken.Text)
	}

	var underlyingTypeConstraint *TypeConstraintAst
	oldMode := p.tok.Mode
	p.setTokenizerMode(ModeSignature)
	colonToken := p.peekToken()
	if colonToken.Kind == Colon {
		p.skipToken()
		p.skipNewlines()
		underlyingType, _ := p.typeNameRule(false)
		if underlyingType == nil {
			p.reportIncompleteInput(after(colonToken.Extent), "TypeNameExpected")
		}
		if isInvalidEnumUnderlyingType(underlyingType) {
			p.reportError(underlyingType.Extent(), "InvalidUnderlyingType", underlyingType.Name())
		}
		underlyingTypeConstraint = newTypeConstraintAst(underlyingType.Extent(), underlyingType)
	}

	p.skipNewlines()
	lCurly := p.nextToken()
	if lCurly.Kind != LCurly {
		p.ungetToken(lCurly)
		p.reportIncompleteInput(after(name.Extent()), "MissingTypeBody", enumToken.Kind.Text())
	}

	members := []MemberAst{}
	for {
		member := p.enumMemberRule()
		if member == nil {
			break
		}
		members = append(members, member)
	}

	rCurly := p.nextToken()
	if rCurly.Kind != RCurly {
		p.ungetToken(rCurly)
		p.reportIncompleteInputAt(after(lCurly.Extent), rCurly.Extent, "MissingEndCurlyBrace")
	}

	startExtent := enumToken.Extent
	if len(customAttributes) > 0 {
		startExtent = customAttributes[0].Extent()
	}
	ext := extentOf(startExtent, rCurly.Extent)
	attributes, typeConstraint := splitAttributes(customAttributes)
	if typeConstraint {
		// statementRule reports a type constraint before a keyword statement, so this is never reached.
		p.reportError(startExtent, ErrUnreachable)
	}
	var baseTypes []*TypeConstraintAst
	if underlyingTypeConstraint != nil {
		baseTypes = []*TypeConstraintAst{underlyingTypeConstraint}
	}
	def := newTypeDefinitionAst(ext, name.Value, attributes, members, TypeAttributesEnum, baseTypes)
	p.setTokenizerMode(oldMode)
	return def
}

// enumMemberRule is EnumMemberRule: Name or Name = value; nil when no name starts here.
func (p *parser) enumMemberRule() MemberAst {
	const enumMemberAttributes = PropertyAttributesPublic | PropertyAttributesStatic | PropertyAttributesLiteral

	p.skipNewlines()
	enumeratorName, _ := p.simpleNameRule()
	if enumeratorName == nil {
		return nil
	}

	endExtent := enumeratorName.Extent()
	var initialValueAst ExpressionAst
	oldMode := p.tok.Mode
	p.setTokenizerMode(ModeExpression)
	assignToken := p.peekToken()
	if assignToken.Kind == Equals {
		p.skipToken()
		initialValueAst = p.expressionRule(false)
		if initialValueAst == nil {
			p.reportError(after(assignToken.Extent), "ExpectedValueExpression", assignToken.Kind.Text())
		}
		endExtent = initialValueAst.Extent()
	}
	p.setTokenizerMode(oldMode)

	terminatorToken := p.peekToken()
	if terminatorToken.Kind != NewLine && terminatorToken.Kind != Semi && terminatorToken.Kind != RCurly {
		p.reportIncompleteInput(after(endExtent), "MissingPropertyTerminator")
	}
	p.skipNewlinesAndSemicolons()
	// The extent takes in a semicolon, but not a newline or closing brace.
	if terminatorToken.Kind == Semi {
		endExtent = terminatorToken.Extent
	}
	return newPropertyMemberAst(extentOf(enumeratorName.Extent(), endExtent), enumeratorName.Value, nil, nil, enumMemberAttributes, initialValueAst)
}

// methodDeclarationRule is MethodDeclarationRule: Name(params) [: base(args)] { body }, the base call allowed on
// an instance constructor only. A constructor's body starts with the base constructor call, implicit when none is
// written.
func (p *parser) methodDeclarationRule(functionNameToken *Token, className string, isStaticMethod bool) StatementAst {
	functionName := functionNameToken.Text
	var parameters []*ParameterAst
	var rParen *Token
	lParen := p.peekToken()
	if lParen.Kind == LParen {
		parameters, _, rParen = p.functionParameterDeclarationRule()
	} else {
		p.reportIncompleteInput(after(functionNameToken.Extent), "MissingMethodParameterList")
	}

	isCtor := OrdinalEqualFold(functionName, className)
	var baseCtorCallParams []ExpressionAst
	var baseToken *Token
	var baseCallLastExtent Extent
	if isCtor && !isStaticMethod {
		p.skipNewlines()
		oldMode := p.tok.Mode
		p.setTokenizerMode(ModeSignature)
		colonToken := p.peekToken()
		if colonToken.Kind == Colon {
			p.skipToken()
			p.skipNewlines()
			baseToken = p.peekToken()
			if baseToken.Kind != Base {
				p.reportIncompleteInput(after(colonToken.Extent), "MissingBaseCtorCall")
			}
			p.skipToken()
			p.skipNewlines()
			lParen = p.peekToken()
			// : base { script } is not shorthand for : base({ script }).
			if lParen.Kind != LParen {
				p.reportIncompleteInput(after(baseToken.Extent), "MissingMethodParameterList")
			}
			p.skipToken()
			baseCtorCallParams, baseCallLastExtent = p.invokeParamParenListRule(lParen)
			p.skipNewlines()
		}
		p.setTokenizerMode(oldMode)
		if baseCtorCallParams == nil {
			baseCtorCallParams = []ExpressionAst{}
		}
	}

	lCurly := p.nextToken()
	if lCurly.Kind != LCurly {
		p.ungetToken(lCurly)
		// C# reports only when no earlier error set the error extent, and every such error stops the parse here.
		pos := functionNameToken.Extent
		if rParen != nil {
			pos = rParen.Extent
		}
		p.reportIncompleteInput(after(pos), "MissingFunctionBody")
	}

	var baseCtorCallStatement StatementAst
	if isCtor && !isStaticMethod {
		baseCallExtent, baseKeywordExtent := emptyExtent, emptyExtent
		if baseToken != nil {
			baseCallExtent = extentOf(baseToken.Extent, baseCallLastExtent)
			baseKeywordExtent = baseToken.Extent
		}
		invokeMemberAst := newBaseCtorInvokeMemberExpressionAst(baseKeywordExtent, baseCallExtent, baseCtorCallParams)
		baseCtorCallStatement = newCommandExpressionAst(invokeMemberAst.Extent(), invokeMemberAst, nil)
	}

	oldMode := p.tok.Mode
	p.setTokenizerMode(ModeCommand)
	scriptBlock := p.scriptBlockRule(lCurly, false, baseCtorCallStatement)
	result := newFunctionDefinitionAst(extentOf(functionNameToken.Extent, scriptBlock.Extent()), false, false, functionNameToken, parameters, scriptBlock)
	p.setTokenizerMode(oldMode)
	return result
}
