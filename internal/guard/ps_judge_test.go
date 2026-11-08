// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/psparse"
)

// These tests walk hand-built trees, so the judge is checked before the parser lands; the PowerShell and bash pairs
// that assert the same verdict through the parser live in ps_judge_pairs_test.go.

func psBare(value string) *psparse.StringConstantExpressionAst {
	return &psparse.StringConstantExpressionAst{StringConstantType: psparse.BareWord, Value: value}
}

func psVar(path string) *psparse.VariableExpressionAst {
	return &psparse.VariableExpressionAst{VariablePath: psparse.NewVariablePath(path)}
}

func psCmd(elements ...psparse.CommandElementAst) *psparse.CommandAst {
	return &psparse.CommandAst{CommandElements: elements}
}

func psPipe(elements ...psparse.CommandBaseAst) *psparse.PipelineAst {
	return &psparse.PipelineAst{PipelineElements: elements}
}

func psExprStmt(expr psparse.ExpressionAst) *psparse.PipelineAst {
	return psPipe(&psparse.CommandExpressionAst{Expression: expr})
}

func psScript(statements ...psparse.StatementAst) *psparse.ScriptBlockAst {
	return &psparse.ScriptBlockAst{EndBlock: &psparse.NamedBlockAst{Unnamed: true, Statements: statements}}
}

func judgeTree(t *testing.T, tree *psparse.ScriptBlockAst) shellScript {
	t.Helper()
	var script shellScript
	if err := script.judgePowerShell(tree, nil, 0); err != nil {
		t.Fatalf("judgePowerShell: %v", err)
	}
	return script
}

func refusalIDs(script shellScript) []string {
	ids := make([]string, 0, len(script.Refusals))
	for _, verdict := range script.Refusals {
		ids = append(ids, verdict.RuleID)
	}
	return ids
}

func TestPowerShellJudgeRefusals(t *testing.T) {
	cases := []struct {
		name string
		tree *psparse.ScriptBlockAst
		want string
	}{
		{"method call", psScript(psExprStmt(&psparse.InvokeMemberExpressionAst{Expression: psVar("x"), Member: psBare("Delete")})), "powershell-construct"},
		{"static member", psScript(psExprStmt(&psparse.MemberExpressionAst{Expression: &psparse.TypeExpressionAst{}, Member: psBare("Delete"), Static: true})), "powershell-construct"},
		{"method call inside an argument", psScript(psPipe(psCmd(psBare("Write-Output"), &psparse.ParenExpressionAst{Pipeline: psExprStmt(&psparse.InvokeMemberExpressionAst{Expression: psVar("x"), Member: psBare("Kill")})}))), "powershell-construct"},
		{"function definition", psScript(&psparse.FunctionDefinitionAst{Name: "f", Body: psScript()}), "powershell-construct"},
		{"class definition", psScript(&psparse.TypeDefinitionAst{Name: "C", TypeAttributes: psparse.TypeAttributesClass}), "powershell-construct"},
		{"trailing ampersand", psScript(&psparse.PipelineAst{PipelineElements: []psparse.CommandBaseAst{&psparse.CommandExpressionAst{Expression: psBare("x")}}, Background: true}), "powershell-construct"},
		{"using module", psScript(&psparse.UsingStatementAst{UsingStatementKind: psparse.UsingModule, Name: psBare("m")}), "powershell-construct"},
		{"function drive assignment", psScript(&psparse.AssignmentStatementAst{Left: psVar("function:ls"), Operator: psparse.Equals, Right: &psparse.CommandExpressionAst{Expression: psBare("x")}}), "name-redefinition"},
		{"alias drive assignment", psScript(&psparse.AssignmentStatementAst{Left: psVar("alias:ls"), Operator: psparse.Equals, Right: &psparse.CommandExpressionAst{Expression: psBare("x")}}), "name-redefinition"},
		{"variable as command name", psScript(psPipe(&psparse.CommandAst{CommandElements: []psparse.CommandElementAst{psVar("tool")}, InvocationOperator: psparse.Ampersand})), "dynamic-command"},
		{"stop parsing", psScript(psPipe(psCmd(psBare("tool"), psBare("--%"), psBare("a b")))), "powershell-construct"},
		{"requires a module", &psparse.ScriptBlockAst{ScriptRequirements: &psparse.ScriptRequirements{RequiredModules: []string{"m"}}}, "powershell-construct"},
		{"refusal inside an else branch", psScript(&psparse.IfStatementAst{
			Clauses:    []psparse.IfClause{{Condition: psExprStmt(psVar("true")), Body: &psparse.StatementBlockAst{}}},
			ElseClause: &psparse.StatementBlockAst{Statements: []psparse.StatementAst{&psparse.FunctionDefinitionAst{Name: "g", Body: psScript()}}},
		}), "powershell-construct"},
		{"refusal inside a script block argument", psScript(psPipe(psCmd(psBare("ForEach-Object"), &psparse.ScriptBlockExpressionAst{ScriptBlock: psScript(psExprStmt(&psparse.InvokeMemberExpressionAst{Expression: psVar("_"), Member: psBare("Delete")}))}))), "powershell-construct"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := refusalIDs(judgeTree(t, tc.tree))
			found := false
			for _, id := range got {
				found = found || id == tc.want
			}
			if !found {
				t.Fatalf("refusals %v, want %s", got, tc.want)
			}
		})
	}
}

func TestPowerShellJudgePasses(t *testing.T) {
	cases := []struct {
		name string
		tree *psparse.ScriptBlockAst
	}{
		{"environment read", psScript(psExprStmt(psVar("env:PATH")))},
		{"environment write", psScript(&psparse.AssignmentStatementAst{Left: psVar("env:X"), Operator: psparse.Equals, Right: &psparse.CommandExpressionAst{Expression: psBare("y")}})},
		{"member access", psScript(psExprStmt(&psparse.MemberExpressionAst{Expression: psVar("_"), Member: psBare("Length")}))},
		{"arithmetic", psScript(psExprStmt(&psparse.BinaryExpressionAst{Left: &psparse.ConstantExpressionAst{Value: int32(1)}, Right: &psparse.ConstantExpressionAst{Value: int32(2)}}))},
		{"using namespace", psScript(&psparse.UsingStatementAst{UsingStatementKind: psparse.UsingNamespace, Name: psBare("System.IO")})},
		{"script block invoked with &", psScript(psPipe(&psparse.CommandAst{CommandElements: []psparse.CommandElementAst{&psparse.ScriptBlockExpressionAst{ScriptBlock: psScript()}}, InvocationOperator: psparse.Ampersand}))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script := judgeTree(t, tc.tree)
			if len(script.Refusals) != 0 || len(script.Calls) != 0 || len(script.Redirects) != 0 {
				t.Fatalf("got refusals %v, calls %v, redirects %v; want none", refusalIDs(script), script.Calls, script.Redirects)
			}
		})
	}
}

func TestPowerShellJudgeRecords(t *testing.T) {
	t.Run("drive variable write is set-content", func(t *testing.T) {
		script := judgeTree(t, psScript(&psparse.AssignmentStatementAst{Left: &psparse.VariableExpressionAst{VariablePath: psparse.NewVariablePath(`C:\x.txt`)}, Operator: psparse.Equals, Right: &psparse.CommandExpressionAst{Expression: psBare("y")}}))
		if len(script.Calls) != 1 || script.Calls[0].Name != "set-content" {
			t.Fatalf("calls %v, want one set-content", script.Calls)
		}
	})
	t.Run("drive variable read is get-content", func(t *testing.T) {
		script := judgeTree(t, psScript(psExprStmt(&psparse.VariableExpressionAst{VariablePath: psparse.NewVariablePath(`C:\x.txt`)})))
		if len(script.Calls) != 1 || script.Calls[0].Name != "get-content" {
			t.Fatalf("calls %v, want one get-content", script.Calls)
		}
	})
	t.Run("redirection to a file", func(t *testing.T) {
		script := judgeTree(t, psScript(psPipe(&psparse.CommandExpressionAst{Expression: psBare("x"), Redirections: []psparse.RedirectionAst{&psparse.FileRedirectionAst{FromStream: psparse.RedirectionStream(1), Location: psBare("out.txt")}}})))
		if len(script.Redirects) != 1 || script.Redirects[0] != (shellRedirect{Path: "out.txt", Write: true}) {
			t.Fatalf("redirects %v, want a write to out.txt", script.Redirects)
		}
	})
	t.Run("redirection to null", func(t *testing.T) {
		script := judgeTree(t, psScript(psPipe(&psparse.CommandExpressionAst{Expression: psBare("x"), Redirections: []psparse.RedirectionAst{&psparse.FileRedirectionAst{Location: psVar("null")}}})))
		if len(script.Redirects) != 0 {
			t.Fatalf("redirects %v, want none", script.Redirects)
		}
	})
	t.Run("clean block is also the 5.1 command", func(t *testing.T) {
		script := judgeTree(t, &psparse.ScriptBlockAst{CleanBlock: &psparse.NamedBlockAst{}})
		if len(script.Calls) != 1 || script.Calls[0].Name != "clean" {
			t.Fatalf("calls %v, want the clean command", script.Calls)
		}
	})
}

func TestPowerShellLegacyDecode(t *testing.T) {
	cases := []struct {
		raw    string
		kind   psparse.StringConstantType
		nested []string
		want   string
	}{
		{"\"a`eb\"", psparse.DoubleQuoted, nil, "aeb"},
		{"\"`u{41}\"", psparse.DoubleQuoted, nil, "u{41}"},
		{"\"x`ty\"", psparse.DoubleQuoted, nil, "x\ty"},
		{"\"say \"\"hi\"\" `e\"", psparse.DoubleQuoted, nil, "say \"hi\" e"},
		{"a`eb", psparse.BareWord, nil, "aeb"},
		{"\"$x`e$($y)\"", psparse.DoubleQuoted, []string{"$x", "$($y)"}, "$xe$($y)"},
		{"@\"\nline`e\n\"@", psparse.DoubleQuotedHereString, nil, "linee"},
	}
	for _, tc := range cases {
		if got := psLegacyDecode(tc.raw, tc.kind, tc.nested); got != tc.want {
			t.Errorf("psLegacyDecode(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestPowerShellRefusalGuidance(t *testing.T) {
	script := judgeTree(t, psScript(psPipe(psCmd(psBare("tool"), psBare("--%"), psBare("x")))))
	if len(script.Refusals) != 1 {
		t.Fatalf("refusals %v, want one", refusalIDs(script))
	}
	message := script.Refusals[0].Message()
	for _, want := range []string{"powershell-construct", "--%", "quoted PowerShell strings"} {
		if !strings.Contains(message, want) {
			t.Errorf("message %q lacks %q", message, want)
		}
	}
	if strings.Contains(message, "--") && strings.Contains(message, " -- ") {
		t.Errorf("message %q holds a double dash used as a dash", message)
	}
}

func TestPowerShellNonASCIIKeywordToken(t *testing.T) {
	cases := []struct {
		tokens []*psparse.Token
		want   string
	}{
		{[]*psparse.Token{{Kind: psparse.Switch, Text: "\u017Fwitch"}}, "\u017Fwitch"},
		{[]*psparse.Token{{Kind: psparse.Switch, Text: "SWITCH"}, {Kind: psparse.If, Text: "if"}}, ""},
		{[]*psparse.Token{{Kind: psparse.Identifier, Text: "\u0131f"}}, ""},
		{[]*psparse.Token{{Kind: psparse.StringExpandable, Text: `"$(x)"`, NestedTokens: []*psparse.Token{{Kind: psparse.If, Text: "\u0131f"}}}}, "\u0131f"},
	}
	for _, tc := range cases {
		got := ""
		if token := psNonASCIIKeyword(tc.tokens); token != nil {
			got = token.Text
		}
		if got != tc.want {
			t.Errorf("psNonASCIIKeyword = %q, want %q", got, tc.want)
		}
	}
}
