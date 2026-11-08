// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The schema is derived from the port's declared fields. The reference process
// reads those fields from the real AST, so a missing or misspelled field fails
// rather than silently comparing a smaller common subset.
type oracleSpec struct {
	Kind    string
	Name    string      `json:",omitempty"`
	Element *oracleSpec `json:",omitempty"`
}

type oracleSchema struct {
	Nodes   map[string]map[string]oracleSpec
	Objects map[string]map[string]oracleSpec
}

var oracleAstType = reflect.TypeFor[Ast]()
var oracleTypeNameType = reflect.TypeFor[ITypeName]()

func makeOracleSchema() oracleSchema {
	s := oracleSchema{Nodes: make(map[string]map[string]oracleSpec), Objects: make(map[string]map[string]oracleSpec)}
	// Interfaces do not enumerate their implementations. Keep this list explicit
	// so a new concrete AST requires a deliberate addition to the oracle.
	for _, node := range []Ast{
		&ScriptBlockAst{}, &ParamBlockAst{}, &NamedBlockAst{}, &NamedAttributeArgumentAst{},
		&AttributeAst{}, &TypeConstraintAst{}, &ParameterAst{}, &StatementBlockAst{},
		&TypeDefinitionAst{}, &UsingStatementAst{}, &PropertyMemberAst{}, &FunctionMemberAst{},
		&FunctionDefinitionAst{}, &IfStatementAst{}, &DataStatementAst{}, &ForEachStatementAst{},
		&ForStatementAst{}, &DoWhileStatementAst{}, &DoUntilStatementAst{}, &WhileStatementAst{},
		&SwitchStatementAst{}, &CatchClauseAst{}, &TryStatementAst{}, &TrapStatementAst{},
		&BreakStatementAst{}, &ContinueStatementAst{}, &ReturnStatementAst{}, &ExitStatementAst{},
		&ThrowStatementAst{}, &PipelineChainAst{}, &PipelineAst{}, &CommandParameterAst{},
		&CommandAst{}, &CommandExpressionAst{}, &MergingRedirectionAst{}, &FileRedirectionAst{},
		&AssignmentStatementAst{}, &BlockStatementAst{}, &TernaryExpressionAst{}, &BinaryExpressionAst{},
		&UnaryExpressionAst{}, &AttributedExpressionAst{}, &ConvertExpressionAst{}, &MemberExpressionAst{},
		&InvokeMemberExpressionAst{}, &BaseCtorInvokeMemberExpressionAst{}, &TypeExpressionAst{},
		&VariableExpressionAst{}, &ConstantExpressionAst{}, &StringConstantExpressionAst{},
		&ExpandableStringExpressionAst{}, &ScriptBlockExpressionAst{}, &ArrayLiteralAst{},
		&HashtableAst{}, &ArrayExpressionAst{}, &ParenExpressionAst{}, &SubExpressionAst{},
		&UsingExpressionAst{}, &IndexExpressionAst{},
	} {
		t := reflect.TypeOf(node).Elem()
		s.Nodes[t.Name()] = s.fields(t)
	}
	return s
}

func (s oracleSchema) fields(t reflect.Type) map[string]oracleSpec {
	fields := make(map[string]oracleSpec)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			for name, spec := range s.fields(f.Type) {
				fields[name] = spec
			}
			continue
		}
		if !f.IsExported() || f.Name == "Ext" {
			continue
		}
		fields[f.Name] = s.spec(f.Type)
	}
	return fields
}

func (s oracleSchema) spec(t reflect.Type) oracleSpec {
	switch {
	case t == reflect.TypeFor[Extent]():
		return oracleSpec{Kind: "extent"}
	case t == reflect.TypeFor[*Token]():
		return oracleSpec{Kind: "token"}
	case t == reflect.TypeFor[*VariablePath]():
		return oracleSpec{Kind: "variable"}
	case t.Implements(oracleTypeNameType):
		return oracleSpec{Kind: "typename"}
	case t.Implements(oracleAstType):
		return oracleSpec{Kind: "node"}
	}
	if t.Kind() == reflect.Pointer {
		return s.spec(t.Elem())
	}
	switch t.Kind() {
	case reflect.Slice:
		elem := s.spec(t.Elem())
		return oracleSpec{Kind: "list", Element: &elem}
	case reflect.Struct:
		if _, ok := s.Objects[t.Name()]; !ok {
			s.Objects[t.Name()] = nil
			s.Objects[t.Name()] = s.fields(t)
		}
		return oracleSpec{Kind: "object", Name: t.Name()}
	case reflect.Interface:
		return oracleSpec{Kind: "constant"}
	case reflect.String:
		return oracleSpec{Kind: "string"}
	case reflect.Bool:
		return oracleSpec{Kind: "bool"}
	default:
		return oracleSpec{Kind: "int"}
	}
}

func oracleExtent(e Extent) map[string]any {
	if e == emptyExtent {
		e = Extent{}
	}
	return map[string]any{"Start": e.Start, "End": e.End}
}

func oracleString(s string) map[string]any {
	units := stringToUTF16(s)
	if units == nil {
		units = []uint16{}
	}
	return map[string]any{"UTF16": units}
}

func oracleToken(t *Token) any {
	if t == nil {
		return nil
	}
	nested := make([]any, 0, len(t.NestedTokens))
	for _, n := range t.NestedTokens {
		nested = append(nested, oracleToken(n))
	}
	x := oracleExtent(t.Extent)
	x["Kind"], x["Flags"], x["NestedTokens"] = t.Kind.String(), uint32(t.Flags), nested
	x["Class"] = t.Class.String()
	return x
}

func oracleValue(v reflect.Value, spec oracleSpec, schema oracleSchema) any {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			if spec.Kind == "list" {
				return []any{}
			}
			return nil
		}
		if spec.Kind == "node" || spec.Kind == "typename" || spec.Kind == "token" || spec.Kind == "variable" {
			break
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return nil
	}
	switch spec.Kind {
	case "node":
		a := v.Interface().(Ast)
		t := reflect.TypeOf(a).Elem()
		x := oracleExtent(a.Extent())
		x["Type"] = t.Name()
		for name, field := range schema.Nodes[t.Name()] {
			x[name] = oracleValue(reflect.ValueOf(a).Elem().FieldByName(name), field, schema)
		}
		return x
	case "typename":
		n := v.Interface().(ITypeName)
		x := oracleExtent(n.Extent())
		x["Type"] = reflect.TypeOf(n).Elem().Name()
		x["Name"], x["FullName"], x["AssemblyName"] = oracleString(n.Name()), oracleString(n.FullName()), oracleString(n.AssemblyName())
		x["IsArray"], x["IsGeneric"] = n.IsArray(), n.IsGeneric()
		return x
	case "variable":
		return map[string]any{"UserPath": oracleString(v.Interface().(*VariablePath).UserPath)}
	case "token":
		return oracleToken(v.Interface().(*Token))
	case "extent":
		return oracleExtent(v.Interface().(Extent))
	case "list":
		x := make([]any, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			x = append(x, oracleValue(v.Index(i), *spec.Element, schema))
		}
		return x
	case "object":
		x := make(map[string]any)
		for name, field := range schema.Objects[spec.Name] {
			x[name] = oracleValue(v.FieldByName(name), field, schema)
		}
		return x
	case "constant":
		return oracleConstant(v.Interface())
	case "string":
		return oracleString(v.String())
	default:
		return v.Interface()
	}
}

func oracleConstant(v any) any {
	name := ""
	value := fmt.Sprint(v)
	switch n := v.(type) {
	case nil, bool:
		return v
	case string:
		return oracleString(n)
	case int8:
		name = "SByte"
	case uint8:
		name = "Byte"
	case int16:
		name = "Int16"
	case uint16:
		name = "UInt16"
	case int32:
		name = "Int32"
	case uint32:
		name = "UInt32"
	case int64:
		name = "Int64"
	case uint64:
		name = "UInt64"
	case float32:
		name, value = "Single", strconv.FormatFloat(float64(n), 'g', -1, 32)
	case float64:
		name, value = "Double", strconv.FormatFloat(n, 'g', -1, 64)
	case *big.Int:
		name, value = "BigInteger", n.String()
	case big.Int:
		name, value = "BigInteger", n.String()
	case Decimal:
		name = "Decimal"
		value = new(big.Rat).SetFrac(n.Unscaled, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n.Scale)), nil)).RatString()
	default:
		panic(fmt.Sprintf("oracle: unhandled constant %T", v))
	}
	return map[string]any{"Type": name, "Value": value}
}

type oracleError struct {
	ID         string
	Start, End int
	Incomplete bool
}

type oracleResult struct {
	Tree   any
	Tokens []any
	Errors []oracleError
}

type oracleResponse struct {
	Version       string
	Results       []oracleResult
	ProtocolError string
	Extractions   [][]struct {
		Offset int
		UTF16  []uint16
	}
}

// One process per test/fuzz worker amortizes PowerShell startup. A deadline kills
// a stuck protocol instead of leaving a fuzz worker blocked on a pipe forever.
type oracleProcess struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	cancel     context.CancelFunc
	in         io.WriteCloser
	out        *bufio.Reader
	stderr     bytes.Buffer
	schema     oracleSchema
	sentSchema bool
}

func startOracle(t testing.TB) *oracleProcess {
	t.Helper()
	program := os.Getenv("LGRASS_PSPARSE_ORACLE_EXE")
	if program == "" {
		program = "pwsh"
	}
	path, err := exec.LookPath(program)
	if err != nil {
		if os.Getenv("LGRASS_PSPARSE_ORACLE_REQUIRED") != "" {
			t.Fatalf("oracle: required PowerShell unavailable: %v", err)
		}
		t.Skipf("oracle: PowerShell unavailable: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	o := &oracleProcess{cancel: cancel, schema: makeOracleSchema()}
	o.cmd = exec.CommandContext(ctx, path, "-NoLogo", "-NoProfile", "-NonInteractive", "-File", filepath.Join("oracle", "dump.ps1"))
	o.cmd.Stderr = &o.stderr
	o.in, err = o.cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	out, err := o.cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	o.out = bufio.NewReader(out)
	if err := o.cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		_ = o.in.Close()
		timer := time.AfterFunc(5*time.Second, cancel)
		err := o.cmd.Wait()
		timer.Stop()
		cancel()
		if err != nil {
			t.Errorf("oracle: process exit: %v; stderr: %s", err, o.stderr.String())
		}
	})
	return o
}

func (o *oracleProcess) batch(inputs []string) (oracleResponse, error) {
	return o.request(inputs, "Parse")
}

func (o *oracleProcess) request(inputs []string, operation string) (oracleResponse, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	units := make([][]uint16, len(inputs))
	for i, input := range inputs {
		units[i] = stringToUTF16(input)
		if units[i] == nil {
			units[i] = []uint16{}
		}
	}
	request := map[string]any{"Inputs": units, "Operation": operation}
	if !o.sentSchema {
		request["Schema"] = o.schema
		o.sentSchema = true
	}
	timer := time.AfterFunc(60*time.Second, o.cancel)
	defer timer.Stop()
	if err := json.NewEncoder(o.in).Encode(request); err != nil {
		return oracleResponse{}, err
	}
	line, err := o.out.ReadBytes('\n')
	if err != nil {
		return oracleResponse{}, fmt.Errorf("oracle: response: %w", err)
	}
	var response oracleResponse
	if err = json.Unmarshal(line, &response); err != nil {
		return response, fmt.Errorf("oracle: JSON: %w: %.200s", err, line)
	}
	if response.ProtocolError != "" {
		return response, fmt.Errorf("oracle: %s", response.ProtocolError)
	}
	if operation == "Parse" && len(response.Results) != len(inputs) {
		return response, fmt.Errorf("oracle: got %d results for %d inputs", len(response.Results), len(inputs))
	}
	if operation == "Extract" && len(response.Extractions) != len(inputs) {
		return response, fmt.Errorf("oracle: got %d extractions for %d inputs", len(response.Extractions), len(inputs))
	}
	return response, nil
}

type oracleClass string

const (
	oracleMatch            oracleClass = "match"
	oracleBothError        oracleClass = "both-error"
	oracleSemantic         oracleClass = "semantic-only"
	oracleLimit            oracleClass = "limit"
	oracleDynamicKeyword   oracleClass = "dynamic-keyword"
	oracleRelativeAssembly oracleClass = "relative-assembly"
	oracleUnreachable      oracleClass = "unreachable"
	oracleRequires         oracleClass = "requires-pending"
	oracleUnresolvedEnum   oracleClass = "unresolved-enum"
	oracleMismatch         oracleClass = "mismatch"
)

// Reviewed against SemanticChecks.cs at bf5fc434120666e5f1a8d1dfd6ecac7efc17a4be.
// DuplicateNamedArgument is deliberately excluded: Parser.cs also reports it.
var oracleSemanticIDs = map[string]bool{
	"DuplicateFormalParameter":                   true, // SemanticChecks.cs:96
	"VoidTypeConstraintNotAllowed":               true, // :107,801
	"AttributeNotAllowedOnDeclaration":           true, // :187
	"OrderedAttributeOnlyOnHashLiteralNode":      true, // :307,920
	"OnlyOneParameterListAllowed":                true, // :440
	"EmptyCatchNotLast":                          true, // :546
	"ExceptionTypeAlreadyCaught":                 true, // :573
	"ReferenceNeedsToBeByItselfInTypeConstraint": true, // :812
	"InvalidLeftHandSide":                        true, // :871
	"SplattingNotPermitted":                      true, // :1077
	"WorkflowNotSupportedInPowerShellCore":       true, // :451
	"ControlLeavingFinally":                      true, // :659
	"VoidMethodHasReturn":                        true, // :710
	"ReferenceNeedsToBeByItselfInTypeSequence":   true, // :948
	"ReferenceNeedsToBeLastTypeInTypeConversion": true, // :995
	"UsingStatementNotSupported":                 true, // :1319
	"InvalidNamespaceValue":                      true, // :1330
	"KeywordParameterReservedForFutureUse":       true, // :465,500,510
	"UnexpectedKeyword":                          true, // :1174
	"CustomAttributeTypeNotFound":                true, // SymbolResolver.cs:671
	"ModuleNotFoundDuringParse":                  true, // SymbolResolver.cs:580
	"ThrottleLimitRequiresParallelFlag":          true, // :522
	"TypeNotFound":                               true, // SymbolResolver.cs:676
	"OperatorRequiresVariableOrProperty":         true, // :901
	"InvalidUsingExpression":                     true, // :1024
	"NonStaticMemberAccessInStaticMember":        true, // :1103
}

func compareOracle(goResult *Result, goErr error, reference oracleResult, schema oracleSchema) (oracleClass, string) {
	if goErr != nil {
		var e *ParseError
		if !errors.As(goErr, &e) {
			return oracleMismatch, goErr.Error()
		}
		switch e.ID {
		case ErrInputTooLarge, ErrNestingTooDeep:
			return oracleLimit, e.ID
		case ErrDynamicKeywords:
			return oracleDynamicKeyword, e.ID
		case ErrRequiresNotSupported:
			return oracleRequires, e.ID
		}
		if len(reference.Errors) != 0 {
			r := reference.Errors[0]
			return oracleBothError, fmt.Sprintf("first error Go=%s@%d pwsh=%s@%d", e.ID, e.Extent.Start, r.ID, r.Start)
		}
		if e.ID == ErrUnreachable {
			return oracleUnreachable, "Go reached a recovery-only branch on reference success"
		}
		if e.ID == "ErrorLoadingAssembly" && oracleHasRelativeAssembly(reference.Tree, e.Extent) {
			return oracleRelativeAssembly, "relative using assembly requires filesystem access, deliberately omitted"
		}
		return oracleMismatch, "Go rejected reference success: " + goErr.Error()
	}
	if len(reference.Errors) != 0 {
		requireExact := false
		unresolvedEnum := false
		for _, e := range reference.Errors {
			if e.ID == "UnexpectedAttribute" && oracleAttributedSemanticError(reference.Tree, e) {
				requireExact = true
			} else if e.ID == "DuplicateNamedArgument" && oracleDuplicateNamedArgument(reference.Tree, e) {
				requireExact = true
			} else if e.ID == "InvalidUnderlyingType" && oracleUnresolvedEnumError(reference.Tree, e) {
				requireExact, unresolvedEnum = true, true
			} else if !oracleSemanticIDs[e.ID] {
				return oracleMismatch, "unclassified reference error: " + e.ID
			}
		}
		if requireExact {
			if detail := oracleCompareSuccess(goResult, reference, schema); detail != "" {
				return oracleMismatch, "shared-ID semantic shape requires exact parity: " + detail
			}
		}
		if unresolvedEnum {
			return oracleUnresolvedEnum, fmt.Sprint(reference.Errors)
		}
		return oracleSemantic, fmt.Sprint(reference.Errors)
	}
	if detail := oracleCompareSuccess(goResult, reference, schema); detail != "" {
		return oracleMismatch, detail
	}
	return oracleMatch, ""
}

func oracleWalk(value any, match func(map[string]any) bool) bool {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if oracleWalk(item, match) {
				return true
			}
		}
	case map[string]any:
		if match(v) {
			return true
		}
		for _, item := range v {
			if oracleWalk(item, match) {
				return true
			}
		}
	}
	return false
}

func oracleDecodeString(value any) (string, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	units, ok := object["UTF16"].([]any)
	if !ok {
		return "", false
	}
	text := make([]uint16, len(units))
	for i, unit := range units {
		number, ok := unit.(float64)
		if !ok {
			return "", false
		}
		text[i] = uint16(number)
	}
	return utf16ToString(text), true
}

func oracleDuplicateNamedArgument(tree any, e oracleError) bool {
	return oracleWalk(tree, func(node map[string]any) bool {
		if node["Type"] != "AttributeAst" {
			return false
		}
		arguments, ok := node["NamedArguments"].([]any)
		if !ok {
			return false
		}
		seen := make(map[string]bool)
		for _, argument := range arguments {
			item, ok := argument.(map[string]any)
			if !ok {
				return false
			}
			name, ok := oracleDecodeString(item["ArgumentName"])
			if !ok {
				return false
			}
			key := OrdinalUpper(name)
			if seen[key] && item["Start"] == float64(e.Start) && item["End"] == float64(e.End) {
				return true
			}
			seen[key] = true
		}
		return false
	})
}

func oracleUnresolvedEnumError(tree any, e oracleError) bool {
	return oracleWalk(tree, func(node map[string]any) bool {
		if node["Type"] != "TypeDefinitionAst" || node["TypeAttributes"] != float64(TypeAttributesEnum) {
			return false
		}
		bases, ok := node["BaseTypes"].([]any)
		if !ok || len(bases) != 1 {
			return false
		}
		constraint, ok := bases[0].(map[string]any)
		if !ok {
			return false
		}
		typeName, ok := constraint["TypeName"].(map[string]any)
		if !ok {
			return false
		}
		if typeName["Start"] != float64(e.Start) || typeName["End"] != float64(e.End) {
			return false
		}
		name, ok := oracleDecodeString(typeName["Name"])
		if !ok || name == "" {
			return false
		}
		for _, known := range []string{"string", "datetime", "System.String", "System.DateTime"} {
			if OrdinalEqualFold(name, known) {
				return false
			}
		}
		return true
	})
}

func oracleCompareSuccess(goResult *Result, reference oracleResult, schema oracleSchema) string {
	if goResult == nil || goResult.Ast == nil {
		return "Go returned no AST without an error"
	}
	actual := oracleResult{Tree: oracleValue(reflect.ValueOf(goResult.Ast), oracleSpec{Kind: "node"}, schema), Tokens: make([]any, 0, len(goResult.Tokens))}
	for _, token := range goResult.Tokens {
		actual.Tokens = append(actual.Tokens, oracleToken(token))
	}
	if diff := oracleDiff(actual.Tree, reference.Tree); diff != "" {
		return "tree " + diff
	}
	if diff := oracleDiff(actual.Tokens, reference.Tokens); diff != "" {
		return "tokens " + diff
	}
	return ""
}

// UnexpectedAttribute is also a syntax error ID. Only the exact condition from
// SemanticChecks.VisitAttributedExpression is eligible, with full tree/token
// parity additionally required by compareOracle.
func oracleAttributedSemanticError(value any, e oracleError) bool {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if oracleAttributedSemanticError(item, e) {
				return true
			}
		}
	case map[string]any:
		if v["Type"] == "AttributedExpressionAst" {
			attribute, ok := v["Attribute"].(map[string]any)
			if ok && attribute["Start"] == float64(e.Start) && attribute["End"] == float64(e.End) {
				node := v
				for {
					child, ok := node["Child"].(map[string]any)
					if !ok {
						return true
					}
					if child["Type"] == "VariableExpressionAst" {
						break
					}
					if child["Type"] != "AttributedExpressionAst" {
						return true
					}
					node = child
				}
			}
		}
		for _, item := range v {
			if oracleAttributedSemanticError(item, e) {
				return true
			}
		}
	}
	return false
}

func oracleHasRelativeAssembly(value any, extent Extent) bool {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if oracleHasRelativeAssembly(item, extent) {
				return true
			}
		}
	case map[string]any:
		if v["Type"] == "UsingStatementAst" && v["UsingStatementKind"] == float64(UsingAssembly) {
			name, ok := v["Name"].(map[string]any)
			if ok && name["Start"] == float64(extent.Start) && name["End"] == float64(extent.End) {
				encoded, ok := name["Value"].(map[string]any)
				if !ok {
					return false
				}
				units, ok := encoded["UTF16"].([]any)
				if !ok {
					return false
				}
				text := make([]uint16, len(units))
				for i, unit := range units {
					number, ok := unit.(float64)
					if !ok {
						return false
					}
					text[i] = uint16(number)
				}
				path := utf16ToString(text)
				return path != "" && !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "\\") && !strings.Contains(path, ":")
			}
		}
		for _, item := range v {
			if oracleHasRelativeAssembly(item, extent) {
				return true
			}
		}
	}
	return false
}

func oracleDiff(actual, expected any) string {
	// JSON also makes Go enum types and the reference's JSON numbers comparable.
	a, err := json.Marshal(actual)
	if err != nil {
		return err.Error()
	}
	b, err := json.Marshal(expected)
	if err != nil {
		return err.Error()
	}
	var av, bv any
	_ = json.Unmarshal(a, &av)
	_ = json.Unmarshal(b, &bv)
	canonicalOracleNumbers(av)
	canonicalOracleNumbers(bv)
	return oracleFirstDiff(av, bv, "$")
}

func oracleFirstDiff(a, b any, path string) string {
	if reflect.DeepEqual(a, b) {
		return ""
	}
	switch av := a.(type) {
	case map[string]any:
		if bv, ok := b.(map[string]any); ok {
			keys := make([]string, 0, len(av)+len(bv))
			for key := range av {
				keys = append(keys, key)
			}
			for key := range bv {
				if _, ok := av[key]; !ok {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			for _, key := range keys {
				x, xok := av[key]
				y, yok := bv[key]
				if xok != yok {
					return fmt.Sprintf("%s.%s present: Go=%t pwsh=%t", path, key, xok, yok)
				}
				if diff := oracleFirstDiff(x, y, path+"."+key); diff != "" {
					return diff
				}
			}
		}
	case []any:
		if bv, ok := b.([]any); ok {
			if len(av) != len(bv) {
				return fmt.Sprintf("%s length: Go=%d pwsh=%d", path, len(av), len(bv))
			}
			for i := range av {
				if diff := oracleFirstDiff(av[i], bv[i], fmt.Sprintf("%s[%d]", path, i)); diff != "" {
					return diff
				}
			}
		}
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return fmt.Sprintf("%s: Go=%.500s pwsh=%.500s", path, x, y)
}

func compareOracleInput(input string, reference oracleResult, schema oracleSchema) (class oracleClass, detail string, parseErr error) {
	defer func() {
		if r := recover(); r != nil {
			class, detail = oracleMismatch, fmt.Sprintf("panic: %v\n%.3000s", r, debug.Stack())
		}
	}()
	result, err := Parse(input, DefaultLimits)
	class, detail = compareOracle(result, err, reference, schema)
	return class, detail, err
}

func canonicalOracleNumbers(value any) {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			canonicalOracleNumbers(item)
		}
	case map[string]any:
		if text, ok := v["Value"].(string); ok {
			switch v["Type"] {
			case "Double", "Single":
				bits := 64
				if v["Type"] == "Single" {
					bits = 32
				}
				if number, err := strconv.ParseFloat(text, bits); err == nil {
					v["Value"] = strconv.FormatFloat(number, 'g', -1, bits)
				}
			case "Decimal":
				if r, ok := new(big.Rat).SetString(text); ok {
					v["Value"] = r.RatString()
				}
			}
		}
		for _, item := range v {
			canonicalOracleNumbers(item)
		}
	}
}

func TestOracleProtocol(t *testing.T) {
	o := startOracle(t)
	for _, inputs := range [][]string{{"", "Get-Item 'hello'", "'😀'; $x = 1", "if ("}, {"\"$x $(Get-Item foo)\"", "@{a=1;b=2}", "if ($x) {1} else {2}"}} {
		response, err := o.batch(inputs)
		if err != nil {
			t.Fatal(err)
		}
		if response.Version == "" {
			t.Fatal("oracle: missing runtime version")
		}
		for i, result := range response.Results {
			if result.Tree == nil || len(result.Tokens) == 0 {
				t.Fatalf("oracle: empty response for %q", inputs[i])
			}
		}
	}
}

func TestOracleNormalization(t *testing.T) {
	o := startOracle(t)
	for _, test := range []struct {
		input string
		node  Ast
	}{
		{"'hello'", &StringConstantExpressionAst{expressionBase: expressionBase{nodeBase{Ext: Extent{0, 7}}}, StringConstantType: SingleQuoted, Value: "hello"}},
		{"'😀'", &StringConstantExpressionAst{expressionBase: expressionBase{nodeBase{Ext: Extent{0, 4}}}, StringConstantType: SingleQuoted, Value: "😀"}},
		{"\"`u{D800}\"", &StringConstantExpressionAst{expressionBase: expressionBase{nodeBase{Ext: Extent{0, 10}}}, StringConstantType: DoubleQuoted, Value: utf16ToString([]uint16{0xd800})}},
		{"42", &ConstantExpressionAst{expressionBase: expressionBase{nodeBase{Ext: Extent{0, 2}}}, Value: int32(42)}},
		{"1.25d", &ConstantExpressionAst{expressionBase: expressionBase{nodeBase{Ext: Extent{0, 5}}}, Value: Decimal{Unscaled: big.NewInt(125), Scale: 2}}},
		{"$env:PATH", &VariableExpressionAst{expressionBase: expressionBase{nodeBase{Ext: Extent{0, 9}}}, VariablePath: &VariablePath{UserPath: "env:PATH"}}},
	} {
		t.Run(test.input, func(t *testing.T) {
			response, err := o.batch([]string{test.input})
			if err != nil {
				t.Fatal(err)
			}
			name := reflect.TypeOf(test.node).Elem().Name()
			reference := findOracleNode(response.Results[0].Tree, name)
			if reference == nil {
				t.Fatalf("oracle: no %s for %q", name, test.input)
			}
			actual := oracleValue(reflect.ValueOf(test.node), oracleSpec{Kind: "node"}, o.schema)
			if diff := oracleDiff(actual, reference); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func findOracleNode(value any, name string) map[string]any {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if node := findOracleNode(item, name); node != nil {
				return node
			}
		}
	case map[string]any:
		if v["Type"] == name {
			return v
		}
		for _, item := range v {
			if node := findOracleNode(item, name); node != nil {
				return node
			}
		}
	}
	return nil
}

func TestOracleReferenceCorpus(t *testing.T) {
	o := startOracle(t)
	cases := loadOracleCorpus(t)
	upstream := loadOracleUpstream(t, o)
	if !testing.Short() {
		cases = append(cases, upstream...)
	}
	for start := 0; start < len(cases); start += 16 {
		end := min(start+16, len(cases))
		inputs := make([]string, end-start)
		for i := range inputs {
			inputs[i] = cases[start+i].Input
		}
		if _, err := o.batch(inputs); err != nil {
			// Identify the precise source instead of reporting a whole batch.
			for i, input := range inputs {
				if _, err := o.batch([]string{input}); err != nil {
					t.Errorf("%s: %v", cases[start+i].Name, err)
				}
			}
		}
	}
	t.Logf("reference protocol normalized %d cases; extracted %d pinned upstream inputs", len(cases), len(upstream))
}

func requireOracleParser(t testing.TB) {
	t.Helper()
	if _, err := Parse("Get-Item foo", DefaultLimits); err != nil {
		t.Fatalf("oracle: parser rejected readiness command: %v", err)
	}
}

func TestOracleCorpus(t *testing.T) {
	o := startOracle(t)
	requireOracleParser(t)
	cases := loadOracleCorpus(t)
	if !testing.Short() {
		cases = append(cases, loadOracleUpstream(t, o)...)
	}
	runOracleCases(t, o, cases)
}

func runOracleCases(t *testing.T, o *oracleProcess, cases []oracleCase) {
	t.Helper()
	counts := make(map[oracleClass]int)
	firstErrorsEqual := 0
	failures := 0
	var report []map[string]any
	for start := 0; start < len(cases); start += 16 {
		end := min(start+16, len(cases))
		inputs := make([]string, end-start)
		for i := range inputs {
			inputs[i] = cases[start+i].Input
		}
		response, err := o.batch(inputs)
		if err != nil {
			t.Fatal(err)
		}
		for i, reference := range response.Results {
			entry := cases[start+i]
			class, detail, err := compareOracleInput(entry.Input, reference, o.schema)
			counts[class]++
			report = append(report, map[string]any{"Name": entry.Name, "Source": entry.Source, "Input": entry.Input, "Class": class, "Detail": detail})
			if class == oracleBothError {
				var e *ParseError
				if errors.As(err, &e) && e.ID == reference.Errors[0].ID && e.Extent.Start == reference.Errors[0].Start {
					firstErrorsEqual++
				}
			}
			if class == oracleMismatch || class == oracleUnreachable {
				failures++
				if failures <= 20 {
					t.Errorf("%s (%s) [%s]\ninput: %q\n%s", entry.Name, entry.Source, class, entry.Input, detail)
				}
			} else if class == oracleSemantic || class == oracleRelativeAssembly || class == oracleDynamicKeyword || class == oracleLimit {
				t.Logf("%s: %s: %s", class, entry.Name, detail)
			}
		}
	}
	t.Logf("oracle classes: %v; first error ID+offset agreement %d/%d; failures %d", counts, firstErrorsEqual, counts[oracleBothError], failures)
	if failures > 20 {
		t.Errorf("%d additional mismatches omitted", failures-20)
	}
	if path := os.Getenv("LGRASS_PSPARSE_ORACLE_REPORT"); path != "" {
		if t.Name() != "TestOracleCorpus" { path += "." + strings.ReplaceAll(t.Name(), "/", "_") + ".json" }
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOracleComparisonClasses(t *testing.T) {
	schema := makeOracleSchema()
	for _, test := range []struct {
		name   string
		err    error
		errors []oracleError
		want   oracleClass
	}{
		{"both errors", &ParseError{ID: "UnexpectedToken"}, []oracleError{{ID: "MissingExpression"}}, oracleBothError},
		{"reference syntax error", nil, []oracleError{{ID: "MissingExpression"}}, oracleMismatch},
		{"Go syntax error", &ParseError{ID: "UnexpectedToken"}, nil, oracleMismatch},
		{"size limit", &ParseError{ID: ErrInputTooLarge}, nil, oracleLimit},
		{"depth limit", &ParseError{ID: ErrNestingTooDeep}, nil, oracleLimit},
		{"configuration", &ParseError{ID: ErrDynamicKeywords}, nil, oracleDynamicKeyword},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, _ := compareOracle(nil, test.err, oracleResult{Errors: test.errors}, schema)
			if got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}

// Corpus entries carry source locations so a mismatch can be traced back to the
// pinned upstream input rather than an unreviewable generated test name.
type oracleCase struct {
	Name   string
	Source string
	Input  string
}

func loadOracleCorpus(t testing.TB) []oracleCase {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "oracle", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []oracleCase
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var entries []oracleCase
		if err := json.Unmarshal(data, &entries); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, entry := range entries {
			if strings.TrimSpace(entry.Name) == "" || entry.Source == "" {
				t.Fatalf("%s: corpus entry lacks name/source", file)
			}
			cases = append(cases, entry)
		}
	}
	if len(cases) == 0 {
		t.Fatal("oracle: no corpus inputs")
	}
	return cases
}

func loadOracleUpstream(t testing.TB, o *oracleProcess) []oracleCase {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "powershell-v7.6.9", "*.Tests.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []oracleCase
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		response, err := o.request([]string{string(data)}, "Extract")
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, extracted := range response.Extractions[0] {
			cases = append(cases, oracleCase{
				Name:   fmt.Sprintf("%s@%d", filepath.Base(file), extracted.Offset),
				Source: "PowerShell/PowerShell@bf5fc434120666e5f1a8d1dfd6ecac7efc17a4be/" + filepath.ToSlash(file) + ":" + strconv.Itoa(extracted.Offset),
				Input:  utf16ToString(extracted.UTF16),
			})
		}
	}
	if len(cases) == 0 {
		t.Fatal("oracle: no pinned upstream test inputs")
	}
	return cases
}
