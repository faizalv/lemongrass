// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import (
	"regexp"
	"strconv"
	"strings"
)

// The #requires parameters, matched as prefixes the way C# matches them (StartsWith, OrdinalIgnoreCase).
const (
	reqShellID   = "shellid"
	reqPSSnapin  = "pssnapin"
	reqVersion   = "version"
	reqEdition   = "psedition"
	reqAssembly  = "assembly"
	reqModules   = "modules"
	reqElevation = "runasadministrator"
)

// reqHashtable is the Hashtable a constant hashtable argument evaluates to, in source order.
type reqHashtable struct {
	keys, values []any
}

// reqUnsupported unwinds GetScriptRequirements when an argument needs PowerShell's constant folding or its
// LanguagePrimitives conversions, which this port does not model; the caller reports ErrRequiresNotSupported, a
// refusal, rather than guess whether PowerShell would accept the line.
type reqUnsupported struct{ ext Extent }

// psVersion is System.Version: Build and Revision are -1 when absent.
type psVersion struct{ major, minor, build, revision int }

func (v psVersion) String() string {
	s := strconv.Itoa(v.major) + "." + strconv.Itoa(v.minor)
	if v.build >= 0 {
		s += "." + strconv.Itoa(v.build)
		if v.revision >= 0 {
			s += "." + strconv.Itoa(v.revision)
		}
	}
	return s
}

// getScriptRequirements is GetScriptRequirements: each #requires comment is parsed as a command in a nested
// scan and its parameters checked, reporting PowerShell's errors for a malformed line.
func (t *tokenizer) getScriptRequirements() (sr *ScriptRequirements) {
	if t.requiresTokens == nil {
		return nil
	}
	tokens := *t.requiresTokens
	t.requiresTokens = nil

	defer func() {
		if r := recover(); r != nil {
			u, ok := r.(reqUnsupported)
			if !ok {
				panic(r)
			}
			t.parser.reportError(u.ext, ErrRequiresNotSupported)
		}
	}()

	var st reqState
	for _, tok := range tokens {
		ext := Extent{Start: tok.Extent.Start + 1, End: tok.Extent.End}
		nested := &Token{Extent: ext, script: t.src.text[ext.Start:ext.End]}
		state := t.startNestedScan(nested)
		cmd, _ := t.parser.commandRule(false).(*CommandAst)
		t.parser.ungotToken = nil
		t.finishNestedScan(state)

		var snapinName *string
		var snapinVersion *psVersion
		if cmd == nil {
			continue
		}
		if name, ok := cmd.GetCommandName(); !ok || !OrdinalEqualFold(name, "requires") {
			t.parser.reportError(cmd.Extent(), "ScriptRequiresInvalidFormat")
		}
		snapinSpecified := false
		for i := 1; i < len(cmd.CommandElements); i++ {
			if p, ok := cmd.CommandElements[i].(*CommandParameterAst); ok && ordinalHasPrefixFold(reqPSSnapin, p.ParameterName) {
				snapinSpecified = true
				break
			}
		}
		for i := 1; i < len(cmd.CommandElements); i++ {
			if p, ok := cmd.CommandElements[i].(*CommandParameterAst); ok {
				t.handleRequiresParameter(p, cmd.CommandElements, snapinSpecified, &i, &snapinName, &snapinVersion, &st)
			} else {
				t.parser.reportError(cmd.CommandElements[i].Extent(), "ScriptRequiresInvalidFormat")
			}
		}
	}

	sr = &ScriptRequirements{
		RequiredPSEditions:  []string{},
		RequiredAssemblies:  []string{},
		RequiredModules:     []string{},
		IsElevationRequired: st.elevation,
	}
	if st.shellID != nil {
		sr.RequiredApplicationID = *st.shellID
	}
	if st.version != nil {
		sr.RequiredPSVersion = st.version.String()
	}
	if st.editions != nil {
		sr.RequiredPSEditions = st.editions
	}
	if st.assemblies != nil {
		sr.RequiredAssemblies = st.assemblies
	}
	if st.modules != nil {
		sr.RequiredModules = st.modules
	}
	return sr
}

// reqState holds what the #requires lines asked for so far; nil means not specified, as C#'s nulls.
type reqState struct {
	shellID    *string
	version    *psVersion
	editions   []string
	modules    []string
	assemblies []string
	elevation  bool
}

// handleRequiresParameter is HandleRequiresParameter.
func (t *tokenizer) handleRequiresParameter(p *CommandParameterAst, elems []CommandElementAst, snapinSpecified bool,
	index *int, snapinName **string, snapinVersion **psVersion, st *reqState) {
	var argument Ast
	if p.Argument != nil {
		argument = p.Argument
	} else if *index+1 < len(elems) {
		*index++
		argument = elems[*index]
	}

	if ordinalHasPrefixFold(reqElevation, p.ParameterName) {
		st.elevation = true
		if argument != nil {
			t.parser.reportError(p.Extent(), "ParameterCannotHaveArgument", p.ParameterName)
		}
		return
	}
	if argument == nil {
		t.parser.reportError(p.Extent(), "ParameterRequiresArgument", p.ParameterName)
		return
	}
	value, ok := requiresConstant(argument)
	if !ok {
		t.parser.reportError(argument.Extent(), "RequiresArgumentMustBeConstant")
		return
	}

	switch {
	case ordinalHasPrefixFold(reqShellID, p.ParameterName):
		if st.shellID != nil {
			t.parser.reportError(p.Extent(), "ParameterAlreadyBound", "", reqShellID)
		}
		s, isString := value.(string)
		if !isString {
			t.parser.reportError(argument.Extent(), "RequiresInvalidStringArgument", reqShellID)
		}
		st.shellID = &s

	case ordinalHasPrefixFold(reqPSSnapin, p.ParameterName):
		s, isString := value.(string)
		if !isString {
			t.parser.reportError(argument.Extent(), "RequiresInvalidStringArgument", reqPSSnapin)
		}
		if *snapinName != nil {
			t.parser.reportError(p.Extent(), "ParameterAlreadyBound", "", reqPSSnapin)
		}
		if !isPSSnapinIDValid(s) {
			t.parser.reportError(argument.Extent(), "InvalidPSSnapInName")
		}
		*snapinName = &s

	case ordinalHasPrefixFold(reqEdition, p.ParameterName):
		if st.editions != nil {
			t.parser.reportError(p.Extent(), "ParameterAlreadyBound", "", reqEdition)
		}
		for _, arg := range requiresEnumerate(value) {
			s, isString := arg.(string)
			if !isString {
				t.parser.reportError(argument.Extent(), "RequiresInvalidStringArgument", reqEdition)
			}
			if st.editions == nil {
				st.editions = []string{}
			}
			if !OrdinalEqualFold(s, "Desktop") && !OrdinalEqualFold(s, "Core") {
				t.parser.reportError(argument.Extent(), "RequiresPSEditionInvalid", reqEdition)
			}
			for _, e := range st.editions {
				if OrdinalEqualFold(e, s) {
					t.parser.reportError(argument.Extent(), "RequiresPSEditionValueIsAlreadySpecified", reqEdition)
				}
			}
			st.editions = append(st.editions, s)
		}

	case ordinalHasPrefixFold(reqVersion, p.ParameterName):
		text, isString := value.(string)
		if !isString {
			text = t.extentText(argument.Extent())
		}
		version, ok := stringToVersion(text)
		if !ok {
			t.parser.reportError(argument.Extent(), "RequiresVersionInvalid")
		}
		if snapinSpecified {
			if *snapinVersion != nil {
				t.parser.reportError(p.Extent(), "ParameterAlreadyBound", "", reqVersion)
			}
			*snapinVersion = &version
		} else {
			if st.version != nil && *st.version != version {
				t.parser.reportError(p.Extent(), "ParameterAlreadyBound", "", reqVersion)
			}
			st.version = &version
		}

	case ordinalHasPrefixFold(reqAssembly, p.ParameterName):
		for _, arg := range requiresEnumerate(value) {
			s, isString := arg.(string)
			if !isString {
				t.parser.reportError(argument.Extent(), "RequiresInvalidStringArgument", reqAssembly)
			}
			if st.assemblies == nil {
				st.assemblies = []string{}
			}
			seen := false
			for _, a := range st.assemblies {
				seen = seen || a == s
			}
			if !seen {
				st.assemblies = append(st.assemblies, s)
			}
		}

	case ordinalHasPrefixFold(reqModules, p.ParameterName):
		args, isArray := value.([]any)
		if !isArray {
			args = []any{value}
		}
		for _, arg := range args {
			spec, ok := t.moduleSpecification(arg, argument.Extent())
			if !ok {
				t.parser.reportError(argument.Extent(), "RequiresModuleInvalid", "")
			}
			if st.modules == nil {
				st.modules = []string{}
			}
			st.modules = append(st.modules, spec)
		}

	default:
		t.parser.reportError(p.Extent(), "ScriptRequiresInvalidFormat")
	}
}

// requiresEnumerate is the C# test "argumentValue is string || argumentValue is not IEnumerable": a string or
// scalar is one argument; an array is its elements; a hashtable enumerates entries, none of which is a string.
func requiresEnumerate(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case *reqHashtable:
		entries := make([]any, len(x.keys))
		for i := range entries {
			entries[i] = struct{}{}
		}
		return entries
	}
	return []any{v}
}

// snapinIDPattern is IsPSSnapinIdValid's regex; .NET's $ also matches before a final newline.
var snapinIDPattern = regexp.MustCompile(`^[A-Za-z0-9\-_.]*\n?$`)

// isPSSnapinIDValid is PSSnapInInfo.IsPSSnapinIdValid.
func isPSSnapinIDValid(s string) bool { return s != "" && snapinIDPattern.MatchString(s) }

// stringToVersion is Utils.StringToVersion: a bare major gets ".0", then System.Version.TryParse.
func stringToVersion(s string) (psVersion, bool) {
	if s == "" {
		return psVersion{}, false
	}
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return parseVersion(s)
}

// parseVersion is System.Version.TryParse: two to four components, each an int32 that may carry .NET's leading
// and trailing white space and a sign, and none negative.
func parseVersion(s string) (psVersion, bool) {
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return psVersion{}, false
	}
	v := [4]int{-1, -1, -1, -1}
	for i, p := range parts {
		p = strings.Trim(p, "\t\n\v\f\r ")
		n, err := strconv.ParseInt(p, 10, 32)
		if err != nil || n < 0 || (p != "" && (p[0] == '+' || p[0] == '-') && len(p) == 1) {
			return psVersion{}, false
		}
		v[i] = int(n)
	}
	return psVersion{v[0], v[1], v[2], v[3]}, true
}

// requiresConstant is IsConstantValueVisitor.IsConstant with forRequires, followed by ConstantValueVisitor.
// Forms whose value needs evaluation (operators, casts, static members, type literals) are unsupported here.
func requiresConstant(n Ast) (any, bool) {
	switch x := n.(type) {
	case *ConstantExpressionAst:
		return x.Value, true
	case *StringConstantExpressionAst:
		return x.Value, true
	case *VariableExpressionAst:
		if !x.VariablePath.IsVariable() {
			return nil, false
		}
		switch name := x.VariablePath.UnqualifiedPath(); {
		case OrdinalEqualFold(name, "true"):
			return true, true
		case OrdinalEqualFold(name, "false"):
			return false, true
		case OrdinalEqualFold(name, "null"):
			return nil, true
		}
		return nil, false
	case *SubExpressionAst:
		return requiresConstant(x.SubExpression)
	case *UsingExpressionAst:
		return requiresConstant(x.SubExpression)
	case *ParenExpressionAst:
		return requiresConstant(x.Pipeline)
	case *StatementBlockAst:
		if x.Traps != nil || len(x.Statements) != 1 {
			return nil, false
		}
		return requiresConstant(x.Statements[0])
	case *PipelineAst:
		if len(x.PipelineElements) != 1 {
			return nil, false
		}
		ce, ok := x.PipelineElements[0].(*CommandExpressionAst)
		if !ok || len(ce.Redirections) != 0 {
			return nil, false
		}
		return requiresConstant(ce.Expression)
	case *ArrayLiteralAst:
		out := make([]any, 0, len(x.Elements))
		for _, e := range x.Elements {
			v, ok := requiresConstant(e)
			if !ok {
				return nil, false
			}
			out = append(out, v)
		}
		return out, true
	case *HashtableAst:
		h := &reqHashtable{}
		for _, kv := range x.KeyValuePairs {
			k, ok := requiresConstant(kv.Key)
			if !ok {
				return nil, false
			}
			v, ok := requiresConstant(kv.Value)
			if !ok {
				return nil, false
			}
			// Hashtable.Add throws on a null or duplicate key, and IsConstant treats the throw as not constant.
			if k == nil || hashtableHasKey(h, k, x.Extent()) {
				return nil, false
			}
			h.keys = append(h.keys, k)
			h.values = append(h.values, v)
		}
		return h, true
	case *UnaryExpressionAst, *BinaryExpressionAst, *TernaryExpressionAst, *ConvertExpressionAst,
		*MemberExpressionAst, *TypeExpressionAst:
		panic(reqUnsupported{n.Extent()})
	}
	return nil, false
}

// hashtableHasKey is the default Hashtable key equality, object.Equals: the same .NET type and value.
func hashtableHasKey(h *reqHashtable, k any, ext Extent) bool {
	for _, have := range h.keys {
		switch have.(type) {
		case string, bool, int8, uint8, int16, uint16, int32, uint32, int64, uint64, float64:
			switch k.(type) {
			case string, bool, int8, uint8, int16, uint16, int32, uint32, int64, uint64, float64:
				if have == k {
					return true
				}
				continue
			}
		}
		// Decimal, BigInteger, array or hashtable keys: equality this port does not model.
		panic(reqUnsupported{ext})
	}
	return false
}

// guidPattern accepts the Guid formats D, N, B and P; the X format is left unsupported.
var guidPattern = regexp.MustCompile(`^(?:[0-9a-fA-F]{32}|[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|\{[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\}|\([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\))$`)

// moduleSpecification is LanguagePrimitives.ConvertTo<ModuleSpecification> on a #requires -Modules value,
// returning ModuleSpecification.ToString; ok is false where PowerShell's conversion throws.
func (t *tokenizer) moduleSpecification(v any, ext Extent) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", true
	case string:
		return x, x != ""
	case *reqHashtable:
		return moduleSpecificationFromHashtable(x, ext)
	}
	panic(reqUnsupported{ext})
}

// moduleSpecificationFromHashtable is ModuleSpecificationInitHelper, for string-valued entries.
func moduleSpecificationFromHashtable(h *reqHashtable, ext Extent) (string, bool) {
	var name, guid, maxVersion *string
	var version, required *psVersion
	badKeys := false
	for i, k := range h.keys {
		ks, ok := k.(string)
		if !ok {
			panic(reqUnsupported{ext})
		}
		val := h.values[i]
		if val != nil {
			if _, isString := val.(string); !isString {
				panic(reqUnsupported{ext})
			}
		}
		s, _ := val.(string)
		switch {
		case OrdinalEqualFold(ks, "ModuleName"):
			if val == nil {
				name = nil
			} else {
				name = &s
			}
		case OrdinalEqualFold(ks, "ModuleVersion"), OrdinalEqualFold(ks, "RequiredVersion"):
			var pv *psVersion
			if val != nil {
				parsed, ok := parseVersion(s)
				if !ok {
					return "", false
				}
				pv = &parsed
			}
			if OrdinalEqualFold(ks, "ModuleVersion") {
				version = pv
			} else {
				required = pv
			}
		case OrdinalEqualFold(ks, "MaximumVersion"):
			// ModuleCmdletBase.GetMaximumVersion validates the text with rules this port does not model.
			panic(reqUnsupported{ext})
		case OrdinalEqualFold(ks, "GUID"):
			if val != nil {
				if !guidPattern.MatchString(s) {
					panic(reqUnsupported{ext})
				}
				guid = &s
			}
		default:
			badKeys = true
		}
	}
	switch {
	case badKeys:
		return "", false
	case name == nil || *name == "":
		return "", false
	case required == nil && version == nil && maxVersion == nil:
		return "", false
	case required != nil && version != nil:
		return "", false
	}
	var b strings.Builder
	b.WriteString("@{ ModuleName = '" + *name + "'")
	if guid != nil {
		b.WriteString("; Guid = '{" + normalizeGUID(*guid) + "}' ")
	}
	if required != nil {
		b.WriteString("; RequiredVersion = '" + required.String() + "'")
	} else if version != nil {
		b.WriteString("; ModuleVersion = '" + version.String() + "'")
	}
	b.WriteString(" }")
	return b.String(), true
}

// normalizeGUID is Guid.ToString(): the D format in lower case.
func normalizeGUID(s string) string {
	hex := strings.Map(func(r rune) rune {
		if strings.ContainsRune("{}()-", r) {
			return -1
		}
		return r
	}, strings.ToLower(s))
	return hex[0:8] + "-" + hex[8:12] + "-" + hex[12:16] + "-" + hex[16:20] + "-" + hex[20:32]
}
