// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"fmt"
	"strings"

	"github.com/faizalv/lemongrass/internal/psparse"
)

// psDynamicText stands for a value only known at run time, the same text the bash parser writes for a substitution, so
// download-exec and the path checks read both front ends alike.
const psDynamicText = "$(dynamic)"

// psReading is one way a PowerShell edition reads string escapes. Windows PowerShell 5.1 has no `e or `u{..}
// escape and reads them as the letters e and u, so a string holding one has a second reading.
type psReading int

const (
	psModern psReading = iota
	psLegacy
)

// walk visits every statement and construct nested in an expression, recording what they run. It is the only
// place an expression has effects, so a value read twice, once per reading, records its commands once.
func (j *psJudge) walk(expr psparse.ExpressionAst) {
	switch e := expr.(type) {
	case nil, *psparse.StringConstantExpressionAst, *psparse.ConstantExpressionAst, *psparse.TypeExpressionAst:
	case *psparse.ExpandableStringExpressionAst:
		for _, nested := range e.NestedExpressions {
			j.walk(nested)
		}
	case *psparse.VariableExpressionAst:
		j.variableRead(e)
	case *psparse.ArrayLiteralAst:
		for _, element := range e.Elements {
			j.walk(element)
		}
	case *psparse.ParenExpressionAst:
		j.statement(e.Pipeline)
	case *psparse.SubExpressionAst:
		j.statementBlock(e.SubExpression)
	case *psparse.ArrayExpressionAst:
		j.statementBlock(e.SubExpression)
	case *psparse.ScriptBlockExpressionAst:
		j.scriptBlock(e.ScriptBlock)
	case *psparse.HashtableAst:
		for _, pair := range e.KeyValuePairs {
			j.walk(pair.Key)
			j.statement(pair.Value)
		}
	case *psparse.MemberExpressionAst:
		if e.Static {
			j.refuse("powershell-construct", "a static member "+j.text(e), psGuideMethod)
		}
		j.walk(e.Expression)
		j.walkElement(e.Member)
	case *psparse.InvokeMemberExpressionAst:
		j.refuse("powershell-construct", "a method call "+j.text(e), psGuideMethod)
	case *psparse.BaseCtorInvokeMemberExpressionAst:
		j.refuse("powershell-construct", "a base constructor call", psGuideDefinition)
	case *psparse.ConvertExpressionAst:
		j.walk(e.Child)
	case *psparse.AttributedExpressionAst:
		j.walk(e.Child)
	case *psparse.BinaryExpressionAst:
		j.walk(e.Left)
		j.walk(e.Right)
	case *psparse.UnaryExpressionAst:
		j.walk(e.Child)
	case *psparse.TernaryExpressionAst:
		j.walk(e.Condition)
		j.walk(e.IfTrue)
		j.walk(e.IfFalse)
	case *psparse.IndexExpressionAst:
		j.walk(e.Target)
		j.walk(e.Index)
	case *psparse.UsingExpressionAst:
		j.walk(e.SubExpression)
	default:
		j.refuse("powershell-construct", fmt.Sprintf("an expression of type %T", expr), psGuideUnknown)
	}
}

// walkElement walks a member name or other command element that may be an expression.
func (j *psJudge) walkElement(element psparse.CommandElementAst) {
	if expr, ok := element.(psparse.ExpressionAst); ok {
		j.walk(expr)
	}
}

// values is what an expression passes as an argument under reading, one value per array element. It records
// nothing; walk does. A value not known at parse time is Dynamic.
func (j *psJudge) values(expr psparse.ExpressionAst, reading psReading) []psValue {
	switch e := expr.(type) {
	case nil:
		return nil
	case *psparse.StringConstantExpressionAst:
		text := e.Value
		if reading == psLegacy {
			text = j.legacyString(e, e.Value, nil)
		}
		return []psValue{{Text: text, Quoted: e.StringConstantType != psparse.BareWord}}
	case *psparse.ExpandableStringExpressionAst:
		dynamic := false
		for i, nested := range e.NestedExpressions {
			// A leading $HOME counts as written out, as bash's leading $HOME does in literalTarget.
			if !(i == 0 && psHomeVariable(nested) && strings.HasPrefix(e.Value, "$")) {
				dynamic = true
			}
		}
		text := e.Value
		if reading == psLegacy {
			text = j.legacyString(e, e.Value, e.NestedExpressions)
		}
		return []psValue{{Text: text, Dynamic: dynamic, Quoted: e.StringConstantType != psparse.BareWord}}
	case *psparse.ConstantExpressionAst:
		if reading == psLegacy {
			if raw := j.text(e); psSevenOnlyNumber(raw) {
				// 5.1 has no such number and rescans the word as a bare string.
				return []psValue{{Text: raw}}
			}
		}
		return []psValue{{Text: fmt.Sprint(e.Value)}}
	case *psparse.VariableExpressionAst:
		return []psValue{psVariableValue(e)}
	case *psparse.ArrayLiteralAst:
		var out []psValue
		for _, element := range e.Elements {
			out = append(out, j.values(element, reading)...)
		}
		return out
	}
	return []psValue{{Text: psDynamicText, Dynamic: true}}
}

// psVariableValue is a variable as an argument. $true, $false and $null are constants a switch parameter is bound
// with; any other variable is dynamic, written as in the source so the path checks still see a home spelling such
// as $env:USERPROFILE.
func psVariableValue(v *psparse.VariableExpressionAst) psValue {
	path := v.VariablePath
	if path == nil {
		return psValue{Text: psDynamicText, Dynamic: true}
	}
	if !path.IsDriveQualified() {
		for _, constant := range []string{"true", "false", "null"} {
			if psparse.OrdinalEqualFold(path.UserPath, constant) {
				return psValue{Text: "$" + constant}
			}
		}
	}
	return psValue{Text: "$" + path.UserPath, Dynamic: true}
}

// psIsNull reports the $null variable, which as a redirection target discards the output.
func psIsNull(expr psparse.ExpressionAst) bool {
	v, ok := expr.(*psparse.VariableExpressionAst)
	return ok && v.VariablePath != nil && !v.VariablePath.IsDriveQualified() && psparse.OrdinalEqualFold(v.VariablePath.UserPath, "null")
}

// variableRead records a read of a variable on a provider drive, such as ${C:\x}, which reads that item as
// Get-Content would.
func (j *psJudge) variableRead(v *psparse.VariableExpressionAst) {
	path := v.VariablePath
	if path == nil || !path.IsDriveQualified() || psPlainVariableDrive(path.DriveName()) {
		return
	}
	j.cmdletEffect("get-content", "${"+path.UserPath+"}", psPathElements(path.UserPath), false)
}

// psPathElements is a lone -Path argument, for a drive variable read or written as its item.
func psPathElements(path string) []psElement {
	return []psElement{{Param: "Path", Values: []psValue{{Text: path, Quoted: true}}}}
}

// psPlainVariableDrive reports a drive whose items are variables or environment values, which read and write no file.
func psPlainVariableDrive(drive string) bool {
	return psparse.OrdinalEqualFold(drive, "env") || psparse.OrdinalEqualFold(drive, "variable")
}

// psHomeVariable reports $HOME or $env:USERPROFILE, which the path checks expand to the home directory.
func psHomeVariable(expr psparse.ExpressionAst) bool {
	v, ok := expr.(*psparse.VariableExpressionAst)
	if !ok || v.VariablePath == nil {
		return false
	}
	path := v.VariablePath
	if !path.IsDriveQualified() {
		return psparse.OrdinalEqualFold(path.UserPath, "HOME")
	}
	return psparse.OrdinalEqualFold(path.DriveName(), "env") && psparse.OrdinalEqualFold(path.UnqualifiedPath(), "USERPROFILE")
}

// legacyString reads a string the way Windows PowerShell 5.1 does, which differs from value only where the source
// holds `e or `u{..}. Nested expressions are kept as written, as value keeps them.
func (j *psJudge) legacyString(node psparse.Ast, value string, nested []psparse.ExpressionAst) string {
	raw := j.text(node)
	if !strings.Contains(raw, "`e") && !strings.Contains(raw, "`u") {
		return value
	}
	var kind psparse.StringConstantType
	switch n := node.(type) {
	case *psparse.StringConstantExpressionAst:
		kind = n.StringConstantType
	case *psparse.ExpandableStringExpressionAst:
		kind = n.StringConstantType
	}
	nestedTexts := make([]string, 0, len(nested))
	for _, n := range nested {
		nestedTexts = append(nestedTexts, j.text(n))
	}
	return psLegacyDecode(raw, kind, nestedTexts)
}

// psLegacyDecode decodes a string's source text under 5.1's escapes: `0 `a `b `f `n `r `t `v as control characters,
// any other escaped character as itself, a doubled quote in a double-quoted string as one quote. The nested
// expressions, in order, are copied as written where they appear.
func psLegacyDecode(raw string, kind psparse.StringConstantType, nested []string) string {
	units := []rune(raw)
	start, end := psStringBody(units, kind)
	var b strings.Builder
	next := 0
	for pos := start; pos < end; pos++ {
		if next < len(nested) {
			want := []rune(nested[next])
			if len(want) > 0 && psRunesAt(units, pos, want) {
				b.WriteString(nested[next])
				pos += len(want) - 1
				next++
				continue
			}
		}
		r := units[pos]
		switch {
		case r == '`' && pos+1 < end:
			pos++
			if escaped := psLegacyEscape(units[pos]); escaped >= 0 {
				b.WriteRune(escaped)
			} else {
				b.WriteRune(units[pos])
			}
		case kind == psparse.DoubleQuoted && psDoubleQuote(r) && pos+1 < end && psDoubleQuote(units[pos+1]):
			b.WriteRune(r)
			pos++
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// psStringBody returns the rune range between a string's delimiters. A bare word has none; a here-string's are its
// @" line and its "@ line.
func psStringBody(units []rune, kind psparse.StringConstantType) (int, int) {
	switch kind {
	case psparse.DoubleQuoted:
		if len(units) >= 2 {
			return 1, len(units) - 1
		}
		return 0, 0
	case psparse.DoubleQuotedHereString:
		start := 2
		for start < len(units) && units[start] != '\n' {
			start++
		}
		start++
		end := len(units) - 2
		if end > start && units[end-1] == '\n' {
			end--
			if end > start && units[end-1] == '\r' {
				end--
			}
		}
		if start > end {
			return start, start
		}
		return start, end
	}
	return 0, len(units)
}

func psRunesAt(units []rune, pos int, want []rune) bool {
	if pos+len(want) > len(units) {
		return false
	}
	for i, r := range want {
		if units[pos+i] != r {
			return false
		}
	}
	return true
}

// psDoubleQuote reports the characters PowerShell reads as a double quote, the typographic ones included.
func psDoubleQuote(r rune) bool {
	return r == '"' || r == '\u201C' || r == '\u201D' || r == '\u201E'
}

// psLegacyEscape is the character 5.1 makes of an escaped letter, or -1 when the letter stands for itself.
func psLegacyEscape(r rune) rune {
	switch r {
	case '0':
		return 0
	case 'a':
		return '\a'
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	case 'v':
		return '\v'
	}
	return -1
}

// psSevenOnlyNumber reports a number literal pwsh 7.0 added: after an optional sign, a 0b binary or one holding a
// u, ul, y, uy, s, us or n suffix. None of those letters is a hex digit, a multiplier or an exponent, so their
// presence alone marks the form.
func psSevenOnlyNumber(text string) bool {
	for _, sign := range []string{"+", "-", "\u2013", "\u2014", "\u2015"} {
		if strings.HasPrefix(text, sign) {
			text = text[len(sign):]
			break
		}
	}
	if strings.HasPrefix(text, "0b") || strings.HasPrefix(text, "0B") {
		return true
	}
	return strings.ContainsAny(text, "uUyYsSnN")
}
