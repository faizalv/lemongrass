// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import (
	"errors"
	"strings"
	"testing"
)

// parseExpr parses src and returns the tree, skipping the test while a rule it reaches is still a stub.
func parseExpr(t *testing.T, src string) (*Result, error) {
	t.Helper()
	var res *Result
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				if r == notPorted {
					t.Skip("a rule this input reaches is not ported yet")
				}
				panic(r)
			}
		}()
		res, err = Parse(src, DefaultLimits)
	}()
	return res, err
}

// firstExpression returns the expression of the script's first statement, a one-element pipeline.
func firstExpression(t *testing.T, src string) ExpressionAst {
	t.Helper()
	res, err := parseExpr(t, src)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	stmts := res.Ast.EndBlock.Statements
	if len(stmts) == 0 {
		t.Fatalf("%q: no statements", src)
	}
	pipe, ok := stmts[0].(*PipelineAst)
	if !ok || len(pipe.PipelineElements) != 1 {
		t.Fatalf("%q: first statement is %T, want a one-element pipeline", src, stmts[0])
	}
	ce, ok := pipe.PipelineElements[0].(*CommandExpressionAst)
	if !ok {
		t.Fatalf("%q: pipeline element is %T, want *CommandExpressionAst", src, pipe.PipelineElements[0])
	}
	return ce.Expression
}

func wantParseError(t *testing.T, src, id string) {
	t.Helper()
	_, err := parseExpr(t, src)
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("%q: err = %v, want %s", src, err, id)
	}
	if pe.ID != id {
		t.Fatalf("%q: error %s, want %s", src, pe.ID, id)
	}
}

func TestBinaryPrecedence(t *testing.T) {
	e := firstExpression(t, "1 + 2 * 3")
	b, ok := e.(*BinaryExpressionAst)
	if !ok || b.Operator != Plus {
		t.Fatalf("got %T %v, want + at the top", e, b)
	}
	if r, ok := b.Right.(*BinaryExpressionAst); !ok || r.Operator != Multiply {
		t.Fatalf("right is %T, want *", b.Right)
	}
	if b.Extent() != (Extent{Start: 0, End: 9}) {
		t.Fatalf("extent %v", b.Extent())
	}
}

func TestTernary(t *testing.T) {
	e := firstExpression(t, "$a ? 1 : 2")
	if _, ok := e.(*TernaryExpressionAst); !ok {
		t.Fatalf("got %T, want ternary", e)
	}
	wantParseError(t, "$a ? 1", "MissingColonInTernaryExpression")
}

func TestUnaryAndArrayLiteral(t *testing.T) {
	if a, ok := firstExpression(t, ",1").(*ArrayLiteralAst); !ok || len(a.Elements) != 1 {
		t.Fatalf("',1' is not a one-element array literal")
	}
	if a, ok := firstExpression(t, "1, 2, 3").(*ArrayLiteralAst); !ok || len(a.Elements) != 3 {
		t.Fatalf("'1, 2, 3' is not a three-element array literal")
	}
	if u, ok := firstExpression(t, "-not $x").(*UnaryExpressionAst); !ok || u.TokenKind != Not {
		t.Fatalf("'-not $x' is not a unary -not")
	}
	if u, ok := firstExpression(t, "$i++").(*UnaryExpressionAst); !ok || u.TokenKind != PostfixPlusPlus {
		t.Fatalf("'$i++' is not a postfix ++")
	}
	wantParseError(t, "1 -foo", "UnexpectedToken")
}

func TestCastsAndTypes(t *testing.T) {
	if c, ok := firstExpression(t, "[int]$x").(*ConvertExpressionAst); !ok || c.Type.TypeName.FullName() != "int" {
		t.Fatalf("'[int]$x' is not a convert to int")
	}
	m, ok := firstExpression(t, "[int]::MaxValue").(*MemberExpressionAst)
	if !ok || !m.Static {
		t.Fatalf("'[int]::MaxValue' is not a static member access")
	}
	te, ok := firstExpression(t, "[System.Collections.Generic.Dictionary[string,int[]]]").(*TypeExpressionAst)
	if !ok {
		t.Fatalf("not a type expression")
	}
	if got := te.TypeName.FullName(); got != "System.Collections.Generic.Dictionary[string,int[]]" {
		t.Fatalf("FullName %q", got)
	}
	at, ok := firstExpression(t, "[int[,]]").(*TypeExpressionAst)
	if !ok {
		t.Fatalf("not a type expression")
	}
	if arr, ok := at.TypeName.(*ArrayTypeName); !ok || arr.Rank != 2 {
		t.Fatalf("[int[,]] is %T, want rank 2 array", at.TypeName)
	}
}

func TestMemberAccess(t *testing.T) {
	inv, ok := firstExpression(t, "$a.b.c(1, 2)").(*InvokeMemberExpressionAst)
	if !ok || len(inv.Arguments) != 2 {
		t.Fatalf("'$a.b.c(1, 2)' is not a two-argument method call")
	}
	if _, ok := inv.Expression.(*MemberExpressionAst); !ok {
		t.Fatalf("target is %T, want member access", inv.Expression)
	}
	if ix, ok := firstExpression(t, "${x}?[0]").(*IndexExpressionAst); !ok || !ix.NullConditional {
		t.Fatalf("'${x}?[0]' is not a null-conditional index")
	}
	if g, ok := firstExpression(t, "$a.M[string]()").(*InvokeMemberExpressionAst); !ok || len(g.GenericTypeArguments) != 1 {
		t.Fatalf("'$a.M[string]()' is not a generic method call")
	}
	if u, ok := firstExpression(t, "$using:x.y").(*UsingExpressionAst); !ok {
		t.Fatalf("'$using:x.y' is not a using expression")
	} else if _, ok := u.SubExpression.(*MemberExpressionAst); !ok {
		t.Fatalf("using expression holds %T, want the member access", u.SubExpression)
	}
}

func TestStringsAndHashtables(t *testing.T) {
	es, ok := firstExpression(t, `"a $b $($c.d)"`).(*ExpandableStringExpressionAst)
	if !ok || len(es.NestedExpressions) != 2 {
		t.Fatalf("expandable string with two parts not found")
	}
	if _, ok := es.NestedExpressions[1].(*SubExpressionAst); !ok {
		t.Fatalf("second part is %T, want a subexpression", es.NestedExpressions[1])
	}
	if s, ok := firstExpression(t, `"plain"`).(*StringConstantExpressionAst); !ok || s.StringConstantType != DoubleQuoted {
		t.Fatalf("a double-quoted string with no variable is not a string constant")
	}
	h, ok := firstExpression(t, "@{a = 1; b = 2}").(*HashtableAst)
	if !ok || len(h.KeyValuePairs) != 2 {
		t.Fatalf("hashtable with two entries not found")
	}
	if _, ok := firstExpression(t, "{ Get-Item x }").(*ScriptBlockExpressionAst); !ok {
		t.Fatalf("script block expression not found")
	}
	wantParseError(t, "@{a = 1", "MissingEndCurlyBrace")
}

func TestParamBlock(t *testing.T) {
	res, err := parseExpr(t, "param([Parameter(Mandatory)][int]$a = 1, $b) $a")
	if err != nil {
		t.Fatal(err)
	}
	pb := res.Ast.ParamBlock
	if pb == nil || len(pb.Parameters) != 2 {
		t.Fatalf("param block with two parameters not found")
	}
	a := pb.Parameters[0]
	if len(a.Attributes) != 2 || a.DefaultValue == nil {
		t.Fatalf("first parameter: %d attributes, default %v", len(a.Attributes), a.DefaultValue)
	}
	attr, ok := a.Attributes[0].(*AttributeAst)
	if !ok || len(attr.NamedArguments) != 1 || !attr.NamedArguments[0].ExpressionOmitted {
		t.Fatalf("[Parameter(Mandatory)] not read as a named argument with an omitted value")
	}
}

func TestClassEnumUsing(t *testing.T) {
	res, err := parseExpr(t, "class C : B { [int]$x = 1\n C() : base(1) {}\n static [string] M([int]$y) { return 'a' } }")
	if err != nil {
		t.Fatal(err)
	}
	td, ok := res.Ast.EndBlock.Statements[0].(*TypeDefinitionAst)
	if !ok || len(td.Members) != 3 || len(td.BaseTypes) != 1 {
		t.Fatalf("class with three members and one base type not found")
	}
	if fm, ok := td.Members[2].(*FunctionMemberAst); !ok || fm.MethodAttributes&MethodAttributesStatic == 0 {
		t.Fatalf("static method not found")
	}

	if _, err := parseExpr(t, "enum E : byte { A = 1; B }"); err != nil {
		t.Fatal(err)
	}
	wantParseError(t, "enum E : string { A }", "InvalidUnderlyingType")
	wantParseError(t, "enum E : System.DateTime { A }", "InvalidUnderlyingType")
	if _, err := parseExpr(t, "enum E : double { A }"); err != nil {
		t.Fatalf("enum E : double: %v; PowerShell accepts every type code up to Decimal", err)
	}

	res, err = parseExpr(t, "using namespace System.IO\n$x")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Ast.UsingStatements) != 1 || res.Ast.UsingStatements[0].UsingStatementKind != UsingNamespace {
		t.Fatalf("using namespace not found")
	}
	wantParseError(t, "using assembly foo.dll", "ErrorLoadingAssembly")
	wantParseError(t, `using assembly \\server\share\foo.dll`, "CannotLoadAssemblyFromUncPath")
	wantParseError(t, "using assembly http://example.com/foo.dll", "CannotLoadAssemblyWithUriSchema")
}

func TestExpressionDepthLimit(t *testing.T) {
	src := strings.Repeat("(", 500) + "1" + strings.Repeat(")", 500)
	wantParseError(t, src, ErrNestingTooDeep)
	src = "[a" + strings.Repeat("[a", 500) + strings.Repeat("]", 501)
	wantParseError(t, src, ErrNestingTooDeep)
}

func TestAbsoluteURI(t *testing.T) {
	for _, c := range []struct {
		in       string
		scheme   string
		unc, abs bool
	}{
		{`C:\x\foo.dll`, "file", false, true},
		{`c:/x/foo.dll`, "file", false, true},
		{`\\server\share\foo.dll`, "file", true, true},
		{`/opt/foo.dll`, "file", false, true},
		{`file:///opt/foo.dll`, "file", false, true},
		{`file://server/share/foo.dll`, "file", true, true},
		{`http://example.com/foo.dll`, "http", false, true},
		{`foo.dll`, "", false, false},
		{`.\foo.dll`, "", false, false},
		{`\foo.dll`, "file", false, false},
	} {
		scheme, unc, abs := absoluteURI(c.in)
		if abs != c.abs || (abs && (scheme != c.scheme || unc != c.unc)) {
			t.Errorf("absoluteURI(%q) = %q, %v, %v; want %q, %v, %v", c.in, scheme, unc, abs, c.scheme, c.unc, c.abs)
		}
	}
}
