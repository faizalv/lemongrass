// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import (
	"strconv"
	"testing"
)

// stmtOracleInputs exercise the statement half of the parser: every statement rule, named blocks, pipelines,
// chains, redirections, command arguments, --% and the invocation operators, each in valid forms and in the
// malformed forms that reach each error branch.
var stmtOracleInputs = []string{
	// Realistic calls.
	"Get-ChildItem -Path . -Recurse | Where-Object { $_.Length -gt 1MB } | Select-Object -First 5",
	"$files = Get-ChildItem; if ($files) { ($files | Get-Content | Measure-Object -Line).Lines } else { 0 }",
	"$t = (Resolve-Path x).Path; if ($t -ne 'p') { throw 'no' }; Remove-Item -LiteralPath $t -Recurse -Force",
	"Get-Item foo 2>&1 | Out-File x.txt -Append",
	"New-Item -ItemType Directory -Path \"C:\\x y\" -Force | Out-Null",

	// Statement lists, separators and continuation.
	"", " ", "\n", "# only a comment", "a; b; c", "a\n\n;;b", "a `\n b", "a\n| b", "a |\n b", "a |\n\n| b",
	"a | ", "| a", "a | | b", "a | 1", "1 | a", "}", ")", "a }", "a )",

	// Chains and background jobs.
	"a && b || c", "a && b &", "a &", "a | b &", "a; b &", "a ||", "a && ", "a && &", "a && .", "a & && b",
	"a && b ||\n c", "1 && 2", "$x && $y",

	// Redirections.
	"cmd 2>&1 > out.txt", "cmd *> $null", "cmd 3>> log", "cmd 2>&1 2>&1", "cmd 1>&2", "cmd < in", "cmd >",
	"$x > f", "1 > f 2>&1", "cmd > a > b", "cmd 6> x 5> y 4> z", "cmd *>&1", "> f", "2>&1",

	// Invocation operators and command names.
	"& 'C:\\a b\\c.exe' -x 1", ". ./script.ps1", "& $cmd @args", "& { a }", "&", ".", "& $x.y", ". { a } b",
	"&$x", "& foo$bar", "cmd foo$bar", "cmd if else", "cmd -", "cmd @{a=1} @(1)", "cmd $(a) \"x$(b)y\"",
	"-x a", "--x", "cmd -x:1 -y: 2 -z:", "cmd -x:a,b", "cmd a, b ,c", "cmd a,", "cmd ,a", "cmd a,,b",
	"cmd -- -x --", "cmd --% a \"b\" %VAR% | c", "cmd '--%' x y", "cmd \"--%\" x; y", "cmd --%", "cmd -x --% y",
	"cmd 1 2.5 0x10 1kb", "cmd 'a' \"b\" @'\nx\n'@", "cmd [a] (b) {c}", "cmd a=b c:d", "cmd ./a/b ~/c",

	// Functions, filters and named blocks.
	"function f { }", "function f($a, [int]$b = 2) { $a }", "filter g { $_ }", "function f { param($x) begin {} process {} end {} }",
	"function if { }", "function f", "function", "function f (", "function f ($a", "function f () a",
	"function f { begin {} begin {} }", "function f { begin {} x }", "function f { begin }", "function 'a' {}",
	"function f-g.h { }", "function global:f { }", "function f\n{ }", "function f { dynamicparam {} clean {} }",
	"begin { }", "begin { } end { }", "clean { a }", "process { a } x", "{ dynamicparam {} clean {} }",
	"param($a) begin { $a }", "[CmdletBinding()] param($a) a", "param($a)", "param(", "{ param($a) $a }",
	"{ }", "{}", "{\n }", "{\n}", "{ \n\t}", "{ a", "{ a }\n{ b }", "workflow w { }", "workflow w { parallel { a } }",

	// trap, try, catch, finally.
	"trap { continue }", "trap [System.Exception] { 'x' }", "trap", "trap [int]", "trap [Obsolete()] { a }",
	"try { a } catch { b }", "try { a } catch [IO.IOException], [System.Exception] { b } finally { c }",
	"try { a }", "try { } catch [x], { }", "try { } catch [x]", "try", "try { } finally", "try { } finally { }",
	"try { } catch [Obsolete()] { }", "try { } catch { } catch [int] { }", "try { } catch [a],\n[b] { }",
	"try\n{ }\ncatch\n{ }\nfinally\n{ }",

	// data sections.
	"data { 'x' }", "data name -SupportedCommand ConvertFrom-StringData, Get-Thing { 'x' }", "data -x { }",
	"data -SupportedCommand { }", "data", "data name", "data -Supp a,", "data name -Supp a { }",

	// if.
	"if ($a) { } elseif ($b) { } else { }", "if", "if (", "if ($a", "if ($a)", "if ($a) {} else", "if ()",
	"if ($a) {} elseif", "if ($a) {}\nelse {}", "if ($a) {}\n\nelseif ($b) {}", "@{ a = if (1) {}\n b = 10 }",
	"if (a | b) { c }", "if ($a = 1) { }", "if\n(\n$a\n)\n{\n}",

	// switch.
	"switch -regex -casesensitive ($x) { 'a' { 1 } default { 2 } }", "switch -file f.txt { a { } }",
	"switch -file", "switch -wildcard -exact (1) {1{}}", "switch -bad (1) {}", "switch (1) { default {} default {} }",
	"switch (1) { }", "switch (1)", "switch -file x (1) {}", "switch (1) { a }", "switch (1) { a {} ",
	"switch -parallel (1) {1{}}", "switch (\"abc\")  ", "switch", "switch (", "switch (1", "switch ()",
	"switch (1) { {$_ -gt 0} { 'p' } 2 { } -3 { } $x { } (1+1) { } }", "switch (1) { a {}; b {} }",
	"switch -r (1) { a {} }", "switch - (1) { a {} }", "switch (1) {\n a {}\n\n default {}\n}",

	// Loops.
	"foreach ($i in 1..3) { $i }", "foreach -parallel ($i in $x) {}", "foreach -throttlelimit 5 ($i in $x) {}",
	"foreach -throttlelimit", "foreach -bad ($i in $x) {}", "foreach", "foreach (", "foreach ($i",
	"foreach ($i in", "foreach ($i in $x", "foreach ($i in $x)", "foreach (1 in 2) {}", "foreach (@a in $b) {}",
	"for ($i=0; $i -lt 3; $i++) { }", "for (;;) {}", "for (", "for ($i=0", "for () {}", "for ($i;$j;$k)",
	"for ($i\n$j\n$k) {}", "for", "while ($true) { break }", "while", "while (", "while ()", "while ($a",
	"while ($a)", "do { } while ($a)", "do { } until ($a)", "do", "do {}", "do {} while", "do {} while (",
	"do {} while ($a", "do {} while ()", "do {}\nwhile ($a)",

	// Labels and flow control.
	":outer foreach ($a in $b) { break outer }", ":lbl while (1) { continue lbl }", ":lbl a b",
	":lbl switch (1) {a{}}", ":lbl do {} while (1)", ":lbl for (;;) {}", ":lbl", ":lbl\nwhile (1) {}",
	"break", "continue 2", "break $x", "break outer; a", "return", "return 1, 2", "exit 3", "throw \"x\"",
	"throw", "return a | b", "exit", "break -1", "continue (1)",

	// Assignment.
	"$x = 1", "$x += 2", "$a, $b = 1, 2", "$x = if ($y) { 1 }", "$x = a | b", "$x =", "$x = $y = 3",
	"$x ??= 1", "$x = 1 &", "$x = a &", "$x = foreach ($i in 1) { $i }", "$x =\n 1", "1 = 2",

	// Attributes and type constraints before statements.
	"[int]$x = 1", "[Obsolete()] if (1) {}", "[int] if (1) {}", "[int]", "[Obsolete()]", "[a()] function f {}",
	"[a()][b]", "[int][string]$x", "[a()] class C {}", "[int] class C {}", "[x] foreach ($a in $b) {}",
	"[a()] 1", "[int]\n2",

	// Keywords in odd places.
	"from", "define x", "var", "else", "catch", "until", "elseif", "finally", "in", "default", "param",
	"parallel { }", "sequence { }", "inlinescript { }", "workflow w { inlinescript { a } }",
	"workflow w { inlinescript }", "configuration c {}",

	// using.
	"using namespace System", "using module foo", "a; using namespace x", "using namespace System\na",
}

func TestOracleStatements(t *testing.T) {
	o := startOracle(t)
	requireOracleParser(t)
	cases := make([]oracleCase, len(stmtOracleInputs))
	for i, input := range stmtOracleInputs {
		cases[i] = oracleCase{Name: "stmt-" + strconv.Itoa(i), Source: "parser_stmt_test.go", Input: input}
	}
	runOracleCases(t, o, cases)
}

// TestParseStatementsNoPanic holds the statement inputs to the parser's contract without PowerShell: a result or
// a *ParseError, never a panic and never both.
func TestParseStatementsNoPanic(t *testing.T) {
	for _, input := range stmtOracleInputs {
		res, err := Parse(input, DefaultLimits)
		if (res == nil) == (err == nil) {
			t.Errorf("%q: result %v with error %v", input, res, err)
		}
	}
}

// TestParseNestingLimit checks that deep nesting of statements is refused with the Lemongrass error, not a stack
// overflow.
func TestParseNestingLimit(t *testing.T) {
	deep := ""
	for i := 0; i < 500; i++ {
		deep += "if ($a) { "
	}
	_, err := Parse(deep, DefaultLimits)
	pe, ok := err.(*ParseError)
	if !ok || pe.ID != ErrNestingTooDeep {
		t.Fatalf("got %v, want %s", err, ErrNestingTooDeep)
	}
}
