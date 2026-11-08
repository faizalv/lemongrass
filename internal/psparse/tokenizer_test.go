// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import (
	"fmt"
	"strings"
	"testing"
)

// tokenString renders tokens as kind:text pairs, with the value for strings, numbers and parameters.
func tokenString(toks []*Token) string {
	var parts []string
	for _, t := range toks {
		s := fmt.Sprintf("%s[%d,%d]", t.Kind, t.Extent.Start, t.Extent.End)
		switch t.Class {
		case ClassStringLiteralToken, ClassStringExpandableToken:
			s += fmt.Sprintf("=%q", t.StringValue)
		case ClassNumberToken:
			s += fmt.Sprintf("=%v(%T)", t.Value, t.Value)
		case ClassParameterToken:
			s += "=" + t.ParameterName
		case ClassVariableToken:
			s += "=" + t.VariablePath.UserPath
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

func TestScanTokens(t *testing.T) {
	cases := []struct {
		mode TokenizerMode
		in   string
		want string
	}{
		{ModeCommand, "", "EndOfInput[1,1]"},
		{ModeCommand, "Get-Item foo", `Generic[0,8]="Get-Item" Identifier[9,12] EndOfInput[13,13]`},
		{ModeCommand, "gci -Recurse:$true 'a b'", `Identifier[0,3] Parameter[4,13]=Recurse Variable[13,18]=true StringLiteral[19,24]="a b" EndOfInput[25,25]`},
		{ModeCommand, "ls | sort; echo hi", `Identifier[0,2] Pipe[3,4] Identifier[5,9] Semi[9,10] Identifier[11,15] Identifier[16,18] EndOfInput[19,19]`},
		{ModeCommand, "a `\nb # c\n", `Identifier[0,1] LineContinuation[2,4] Identifier[4,5] Comment[6,9] NewLine[9,10] EndOfInput[11,11]`},
		{ModeCommand, "cmd 2>&1 > out.txt", `Identifier[0,3] Redirection[4,8] Redirection[9,10] Generic[11,18]="out.txt" EndOfInput[19,19]`},
		{ModeCommand, "@args $env:PATH", `SplattedVariable[0,5]=args Variable[6,15]=env:PATH EndOfInput[16,16]`},
		{ModeExpression, "1 + 0x10 - 1.5e2 + 10kb", `Number[0,1]=1(int32) Plus[2,3] Number[4,8]=16(int32) Minus[9,10] Number[11,16]=150(float64) Plus[17,18] Number[19,23]=10240(int32) EndOfInput[24,24]`},
		{ModeExpression, "$a -eq 'x'", `Variable[0,2]=a Ieq[3,6] StringLiteral[7,10]="x" EndOfInput[11,11]`},
		{ModeExpression, `"hi $name, $(1+2)"`, `StringExpandable[0,18]="hi $name, $(1+2)" EndOfInput[19,19]`},
		{ModeCommand, "echo \U0001F600 x", "Identifier[0,4] Generic[5,7]=\"\U0001F600\" Identifier[8,9] EndOfInput[10,10]"},
		{ModeCommand, "foreach Foreach", `Foreach[0,7] Foreach[8,15] EndOfInput[16,16]`},
		// The Kelvin sign is a letter, so the word is one identifier and never the keyword break.
		{ModeCommand, "brea\u212a", "Identifier[0,5] EndOfInput[6,6]"},
		{ModeExpression, "0xFFFFFFFF 0b11111111 100y 1d 1.50d", `Number[0,10]=-1(int32) Number[11,21]=-1(int32) Number[22,26]=100(int8) Number[27,29]={1 0}(psparse.Decimal) Number[30,35]={150 2}(psparse.Decimal) EndOfInput[36,36]`},
	}
	for _, c := range cases {
		toks, err := scanTokens(c.in, c.mode)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if got := tokenString(toks); got != c.want {
			t.Errorf("%q:\n got %s\nwant %s", c.in, got, c.want)
		}
	}
}

func TestScanTokensNested(t *testing.T) {
	toks, err := scanTokens(`"a $b $(c ""d"")"`, ModeExpression)
	if err != nil {
		t.Fatal(err)
	}
	n := toks[0].NestedTokens
	if len(n) != 2 || n[0].Kind != Variable || n[1].Class != ClassUnscannedSubExprToken {
		t.Fatalf("nested %s", tokenString(n))
	}
	if n[1].StringValue != `$(c "d")` || len(n[1].SkippedCharOffsets) == 0 {
		t.Fatalf("unscanned %q skipped %v", n[1].StringValue, n[1].SkippedCharOffsets)
	}
}

func TestScanTokensErrors(t *testing.T) {
	for in, id := range map[string]string{
		"'abc":           "TerminatorExpectedAtEndOfString",
		"<# x":           "MissingTerminatorMultiLineComment",
		"${a":            "IncompleteDollarVariableReference",
		"\"`u{110000}\"": "InvalidUnicodeEscapeSequenceValue",
		"@'x":            "UnexpectedCharactersAfterHereStringHeader",
	} {
		_, err := scanTokens(in, ModeCommand)
		pe, ok := err.(*ParseError)
		if !ok || pe.ID != id {
			t.Errorf("%q: %v, want %s", in, err, id)
		}
	}
}

// FuzzScanTokens holds every tokenizer mode to the contract: no panic but abortParse, which scanTokens recovers.
func FuzzScanTokens(f *testing.F) {
	for _, s := range []string{"Get-Item foo", "\"a $b $(c)\"", "@'\nx\n'@", "0x1Fkb 1.5e3d", "${a`u{263a}}", "<# x #> a"} {
		f.Add(s, uint8(0))
	}
	f.Fuzz(func(t *testing.T, input string, mode uint8) {
		_, _ = scanTokens(input, TokenizerMode(mode%4))
	})
}

func TestScriptRequirements(t *testing.T) {
	ok := []struct {
		in   string
		want ScriptRequirements
	}{
		{"#requires -Version 5.1\na", ScriptRequirements{RequiredPSVersion: "5.1"}},
		{"#Requires -Version 7", ScriptRequirements{RequiredPSVersion: "7.0"}},
		{"#requires -RunAsAdministrator", ScriptRequirements{IsElevationRequired: true}},
		{"#requires -PSEdition Core,Desktop", ScriptRequirements{RequiredPSEditions: []string{"Core", "Desktop"}}},
		{"#requires -Modules Az, @{ModuleName='B';RequiredVersion='1.2'}", ScriptRequirements{
			RequiredModules: []string{"Az", "@{ ModuleName = 'B'; RequiredVersion = '1.2' }"}}},
		{"#requires -ShellId x -Assembly a.dll", ScriptRequirements{RequiredApplicationID: "x", RequiredAssemblies: []string{"a.dll"}}},
	}
	for _, c := range ok {
		res, err := Parse(c.in, DefaultLimits)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		got := fmt.Sprintf("%+v", *res.Ast.ScriptRequirements)
		want := c.want
		for _, l := range []*[]string{&want.RequiredPSEditions, &want.RequiredAssemblies, &want.RequiredModules} {
			if *l == nil {
				*l = []string{}
			}
		}
		if w := fmt.Sprintf("%+v", want); got != w {
			t.Errorf("%q:\n got %s\nwant %s", c.in, got, w)
		}
	}
	for in, id := range map[string]string{
		"#requires -Foo 1":                           "ScriptRequiresInvalidFormat",
		"#requires -Version":                         "ParameterRequiresArgument",
		"#requires -Version x":                       "RequiresVersionInvalid",
		"#requires -Version 1\n#requires -Version 2": "ParameterAlreadyBound",
		"#requires -PSEdition Home":                  "RequiresPSEditionInvalid",
		"#requires -Version $x":                      "RequiresArgumentMustBeConstant",
		"#requires -Modules @{ModuleName='a'}":       "RequiresModuleInvalid",
		"#requires -PSSnapin a/b":                    "InvalidPSSnapInName",
		"#requires -Version (1+1)":                   ErrRequiresNotSupported,
	} {
		_, err := Parse(in, DefaultLimits)
		if pe, ok := err.(*ParseError); !ok || pe.ID != id {
			t.Errorf("%q: %v, want %s", in, err, id)
		}
	}
}

func TestBigIntegerToDoubleTruncates(t *testing.T) {
	// pwsh 7.6.9 gives 1.000000001011101e+29 for this literal, one double below the correctly rounded value.
	toks, err := scanTokens("100000000101110110000000000000", ModeExpression)
	if err != nil {
		t.Fatal(err)
	}
	if got := toks[0].Value; got != 1.000000001011101e+29 {
		t.Fatalf("value %v (%T)", got, got)
	}
}
