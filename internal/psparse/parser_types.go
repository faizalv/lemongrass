// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import "strings"

// Type names as Parser.cs reads them in type literals, attributes, casts and base type lists. A tokenizer mode a
// rule switches to is restored on every normal return, as the C# finally blocks do; after an error the parse is
// over, so nothing restores it on the way out.

// newTypeName is the TypeName constructor: a doubled backtick in the name collapses to one.
func newTypeName(ext Extent, name string) *TypeName {
	if strings.Contains(name, "`") {
		name = strings.ReplaceAll(name, "``", "`")
	}
	return &TypeName{Ext: ext, TypeNameText: name}
}

// newTypeNameWithAssembly is TypeName(extent, name, assembly).
func newTypeNameWithAssembly(ext Extent, name, assembly string) *TypeName {
	t := newTypeName(ext, name)
	t.Assembly = assembly
	return t
}

// newOpenGenericTypeName is TypeName(extent, name, genericArgumentCount): the count is kept only when the name
// does not already spell its arity with a backtick, as List`1 does.
func newOpenGenericTypeName(ext Extent, name string, genericArgumentCount int) *TypeName {
	t := newTypeName(ext, name)
	if genericArgumentCount > 0 && !strings.Contains(t.TypeNameText, "`") {
		t.GenericArgumentCount = genericArgumentCount
	}
	return t
}

// typeNameRule is TypeNameRule; name is nil when no type name starts here.
func (p *parser) typeNameRule(allowAssemblyQualifiedNames bool) (name ITypeName, firstTypeNameToken *Token) {
	p.enter()
	defer p.leave()
	oldMode := p.tok.Mode
	// TypeName mode avoids the aggressive argument tokenizing of command mode.
	p.setTokenizerMode(ModeTypeName)
	typeName := p.nextToken()
	if typeName.Kind != Identifier {
		p.ungetToken(typeName)
		p.setTokenizerMode(oldMode)
		return nil, nil
	}
	name = p.finishTypeNameRule(typeName, false, allowAssemblyQualifiedNames)
	p.setTokenizerMode(oldMode)
	return name, typeName
}

// finishTypeNameRule is FinishTypeNameRule; typeName is an Identifier token.
func (p *parser) finishTypeNameRule(typeName *Token, unBracketedGenericArg, allowAssemblyQualifiedNames bool) ITypeName {
	t := p.peekToken()
	if t.Kind == LBracket {
		lbracket := t
		// An array or a generic.
		p.skipToken()
		p.skipNewlines()
		t = p.nextToken()
		switch t.Kind {
		case RBracket, Comma:
			elementType := newTypeName(typeName.Extent, typeName.Text)
			return p.completeArrayTypeName(elementType, elementType, t, unBracketedGenericArg)
		case LBracket, Identifier:
			return p.genericTypeNameRule(typeName, t, unBracketedGenericArg)
		case EndOfInput:
			p.ungetToken(t)
			p.reportIncompleteInput(after(lbracket.Extent), "MissingTypename")
		default:
			p.reportError(t.Extent, "UnexpectedToken", t.Text)
		}
	}

	if t.Kind == Comma && allowAssemblyQualifiedNames && !unBracketedGenericArg {
		p.skipToken()
		assemblyNameSpec := p.tok.getAssemblyNameSpec()
		if strings.TrimSpace(assemblyNameSpec) == "" {
			p.reportError(after(t.Extent), "MissingAssemblyNameSpecification")
		}
		return newTypeNameWithAssembly(extentOf(typeName.Extent, p.tok.currentExtent()), typeName.Text, assemblyNameSpec)
	}

	return newTypeName(typeName.Extent, typeName.Text)
}

// getSingleGenericArgument is GetSingleGenericArgument: an unbracketed name such as string in List[string], or a
// bracketed one such as [string] in List[[string]], which may be assembly qualified.
func (p *parser) getSingleGenericArgument(firstToken *Token) ITypeName {
	if firstToken.Kind == Identifier {
		return p.finishTypeNameRule(firstToken, true, true)
	}

	t := p.nextToken()
	if t.Kind != Identifier {
		p.ungetToken(t)
		p.reportIncompleteInput(after(t.Extent), "MissingTypename")
	}

	typeName := p.finishTypeNameRule(t, false, true)
	rBracket := p.nextToken()
	if rBracket.Kind != RBracket {
		p.ungetToken(rBracket)
		p.reportIncompleteInput(before(rBracket.Extent), "EndSquareBracketExpectedAtEndOfType")
	}
	return typeName
}

// genericTypeArgumentsRule is GenericTypeArgumentsRule. lastToken is the token after the last argument, which
// the caller checks for the closing bracket.
func (p *parser) genericTypeArgumentsRule(firstToken *Token) (args []ITypeName, lastToken *Token) {
	// Nested generic arguments recurse through here without passing typeNameRule.
	p.enter()
	defer p.leave()

	args = append(args, p.getSingleGenericArgument(firstToken))
	for {
		p.skipNewlines()
		lastToken = p.nextToken()
		if lastToken.Kind != Comma {
			break
		}
		p.skipNewlines()
		t := p.peekToken()
		if t.Kind != Identifier && t.Kind != LBracket {
			p.reportIncompleteInput(after(lastToken.Extent), "MissingTypename")
		}
		p.skipToken()
		args = append(args, p.getSingleGenericArgument(t))
	}
	return args, lastToken
}

// genericTypeNameRule is GenericTypeNameRule.
func (p *parser) genericTypeNameRule(genericTypeName, firstToken *Token, unbracketedGenericArg bool) ITypeName {
	genericArguments, rBracketToken := p.genericTypeArgumentsRule(firstToken)
	if rBracketToken.Kind != RBracket {
		p.ungetToken(rBracketToken)
		p.reportIncompleteInput(before(rBracketToken.Extent), "EndSquareBracketExpectedAtEndOfAttribute")
	}

	openGenericType := newOpenGenericTypeName(genericTypeName.Extent, genericTypeName.Text, len(genericArguments))
	result := &GenericTypeName{
		Ext:              extentOf(genericTypeName.Extent, rBracketToken.Extent),
		TypeName:         openGenericType,
		GenericArguments: genericArguments,
	}

	t := p.peekToken()
	if t.Kind == LBracket {
		p.skipToken()
		return p.completeArrayTypeName(result, openGenericType, p.nextToken(), unbracketedGenericArg)
	}

	if t.Kind == Comma && !unbracketedGenericArg {
		p.skipToken()
		assemblyNameSpec := p.tok.getAssemblyNameSpec()
		if assemblyNameSpec == "" {
			p.reportError(after(t.Extent), "MissingAssemblyNameSpecification")
		}
		// The wrapper reads the assembly from the open name on demand, so setting it here qualifies both.
		openGenericType.Assembly = assemblyNameSpec
	}
	return result
}

// completeArrayTypeName is CompleteArrayTypeName: firstTokenAfterLBracket is the ']' or ',' after the '['.
// typeForAssemblyQualification is the plain name an assembly qualifier after the brackets attaches to.
func (p *parser) completeArrayTypeName(elementType ITypeName, typeForAssemblyQualification *TypeName, firstTokenAfterLBracket *Token, unBracketedGenericArg bool) ITypeName {
	for {
		switch firstTokenAfterLBracket.Kind {
		case Comma:
			dim := 1
			t := firstTokenAfterLBracket
			var lastComma *Token
			for {
				lastComma = t
				dim++
				t = p.nextToken()
				if t.Kind != Comma {
					break
				}
			}
			// .NET's MakeArrayType allows at most 32 dimensions.
			if dim > 32 {
				if t.Kind != RBracket {
					p.ungetToken(t)
				}
				p.reportError(extentOf(firstTokenAfterLBracket.Extent, lastComma.Extent), "ArrayHasTooManyDimensions", dim)
			}
			if t.Kind != RBracket {
				p.ungetToken(t)
				p.reportError(after(lastComma.Extent), "EndSquareBracketExpectedAtEndOfAttribute")
			}
			elementType = &ArrayTypeName{Ext: extentOf(elementType.Extent(), t.Extent), ElementType: elementType, Rank: dim}

		case RBracket:
			elementType = &ArrayTypeName{Ext: extentOf(elementType.Extent(), firstTokenAfterLBracket.Extent), ElementType: elementType, Rank: 1}

		case EndOfInput:
			p.ungetToken(firstTokenAfterLBracket)
			p.reportError(before(firstTokenAfterLBracket.Extent), "EndSquareBracketExpectedAtEndOfAttribute")

		default:
			p.reportError(firstTokenAfterLBracket.Extent, "UnexpectedToken", firstTokenAfterLBracket.Text)
		}

		t := p.peekToken()
		// An array inside an unbracketed generic argument cannot be assembly qualified.
		if !unBracketedGenericArg && t.Kind == Comma {
			p.skipToken()
			assemblyName := p.tok.getAssemblyNameSpec()
			if assemblyName == "" {
				p.reportError(after(t.Extent), "MissingAssemblyNameSpecification")
			}
			typeForAssemblyQualification.Assembly = assemblyName
			break
		}
		if t.Kind != LBracket {
			break
		}
		// A jagged array: skip the '[' and read the next rank.
		p.skipToken()
		firstTokenAfterLBracket = p.nextToken()
	}
	return elementType
}

// invalidEnumUnderlyingTypeNames are the names known to resolve to String or DateTime, keyed by OrdinalUpper.
var invalidEnumUnderlyingTypeNames = map[string]bool{
	OrdinalUpper("string"): true, OrdinalUpper("System.String"): true,
	OrdinalUpper("datetime"): true, OrdinalUpper("System.DateTime"): true,
}

// isInvalidEnumUnderlyingType stands in for EnumDefinitionRule's reflection check, with one deliberate divergence.
// C# tests ValidUnderlyingTypeCodes.HasFlag(type.GetTypeCode()) on the non-flags TypeCode enum; the eight integral
// codes OR to 15, so every code from 0 to 15 passes and only DateTime (16) and String (18) fail, besides a name
// that resolves to no type. The parser cannot load types, so it refuses only the names known to be String or
// DateTime and accepts any other, including one PowerShell cannot resolve.
func isInvalidEnumUnderlyingType(name ITypeName) bool {
	tn, ok := name.(*TypeName)
	return ok && invalidEnumUnderlyingTypeNames[OrdinalUpper(tn.TypeNameText)]
}
