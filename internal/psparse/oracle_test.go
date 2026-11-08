// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
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
	Nodes      map[string]map[string]oracleSpec
	Objects    map[string]map[string]oracleSpec
	Precedence map[string]int
}

var oracleAstType = reflect.TypeFor[Ast]()
var oracleTypeNameType = reflect.TypeFor[ITypeName]()

func makeOracleSchema() oracleSchema {
	s := oracleSchema{Nodes: make(map[string]map[string]oracleSpec), Objects: make(map[string]map[string]oracleSpec), Precedence: map[string]int{
		"BinaryPrecedenceLogical": int(FlagBinaryPrecedenceLogical), "BinaryPrecedenceBitwise": int(FlagBinaryPrecedenceBitwise),
		"BinaryPrecedenceComparison": int(FlagBinaryPrecedenceComparison), "BinaryPrecedenceCoalesce": int(FlagBinaryPrecedenceCoalesce),
		"BinaryPrecedenceAdd": int(FlagBinaryPrecedenceAdd), "BinaryPrecedenceMultiply": int(FlagBinaryPrecedenceMultiply),
		"BinaryPrecedenceFormat": int(FlagBinaryPrecedenceFormat), "BinaryPrecedenceRange": int(FlagBinaryPrecedenceRange),
	}}
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
	Tree               any
	Tokens             []any
	Errors             []oracleError
	ReferenceException string
}

type oracleResponse struct {
	Version       string
	Results       []oracleResult
	ProtocolError string
	Precedence    map[string]int
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
	return startOracleProgram(t, program)
}

func startOracleProgram(t testing.TB, program string) *oracleProcess {
	t.Helper()
	path, err := exec.LookPath(program)
	if err != nil {
		if os.Getenv("LGRASS_PSPARSE_ORACLE_REQUIRED") != "" {
			t.Fatalf("oracle: required PowerShell unavailable: %v", err)
		}
		t.Skipf("oracle: PowerShell unavailable: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	o := &oracleProcess{cancel: cancel, schema: makeOracleSchema()}
	args := []string{"-NoLogo", "-NoProfile", "-NonInteractive"}
	if strings.ToLower(filepath.Base(path)) == "powershell.exe" {
		args = append(args, "-ExecutionPolicy", "Bypass")
	}
	args = append(args, "-File", filepath.Join("oracle", "dump.ps1"))
	o.cmd = exec.CommandContext(ctx, path, args...)
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
	oracleMatch                     oracleClass = "match"
	oracleBothError                 oracleClass = "both-error"
	oracleSemantic                  oracleClass = "semantic-only"
	oracleLimit                     oracleClass = "limit"
	oracleDynamicKeyword            oracleClass = "dynamic-keyword"
	oracleRelativeAssembly          oracleClass = "relative-assembly"
	oracleUnreachable               oracleClass = "unreachable"
	oracleRequires                  oracleClass = "requires-pending"
	oracleUnresolvedEnum            oracleClass = "unresolved-enum"
	oracleAssemblyFilesystem        oracleClass = "assembly-filesystem"
	oracleMismatch                  oracleClass = "mismatch"
	oracleRefusedReferenceException oracleClass = "refused-reference-exception"
)

// Reviewed against SemanticChecks.cs at bf5fc434120666e5f1a8d1dfd6ecac7efc17a4be.
// DuplicateNamedArgument is deliberately excluded: Parser.cs also reports it.
var oracleSemanticIDs = map[string]bool{
	"ParameterAttributeArgumentNeedsToBeConstantOrScriptBlock": true, // SemanticChecks.cs:81
	"DuplicateFormalParameter":                                 true, // SemanticChecks.cs:96
	"VoidTypeConstraintNotAllowed":                             true, // :107,801
	"AttributeNotAllowedOnDeclaration":                         true, // :187
	"OrderedAttributeOnlyOnHashLiteralNode":                    true, // :307,920
	"OnlyOneParameterListAllowed":                              true, // :440
	"EmptyCatchNotLast":                                        true, // :546
	"ExceptionTypeAlreadyCaught":                               true, // :573
	"ReferenceNeedsToBeByItselfInTypeConstraint":               true, // :812
	"InvalidLeftHandSide":                                      true, // :871
	"SplattingNotPermitted":                                    true, // :1077
	"WorkflowNotSupportedInPowerShellCore":                     true, // :451
	"ControlLeavingFinally":                                    true, // :659
	"VoidMethodHasReturn":                                      true, // :710
	"ReferenceNeedsToBeByItselfInTypeSequence":                 true, // :948
	"ReferenceNeedsToBeLastTypeInTypeConversion":               true, // :995
	"UsingStatementNotSupported":                               true, // :1319
	"InvalidNamespaceValue":                                    true, // :1330
	"KeywordParameterReservedForFutureUse":                     true, // :465,500,510
	"UnexpectedKeyword":                                        true, // :1174
	"CustomAttributeTypeNotFound":                              true, // SymbolResolver.cs:671
	"ModuleNotFoundDuringParse":                                true, // SymbolResolver.cs:580
	"ThrottleLimitRequiresParallelFlag":                        true, // :522
	"TypeNotFound":                                             true, // SymbolResolver.cs:676
	"OperatorRequiresVariableOrProperty":                       true, // :901
	"InvalidUsingExpression":                                   true, // :1024
	"NonStaticMemberAccessInStaticMember":                      true, // :1103
	"VariableNotLocal":                                         true, // :1093
}

//go:embed oracle/definition-semantic-ids.json
var oracleDefinitionSemanticData []byte

var oracleSemanticEligibility = func() struct{ IDs, UsingModuleIDs map[string][]string } {
	var data struct{ IDs, UsingModuleIDs map[string][]string }
	if err := json.Unmarshal(oracleDefinitionSemanticData, &data); err != nil {
		panic(err)
	}
	return data
}()

var oracleDefinitionSemanticIDs = oracleSemanticEligibility.IDs

func oracleDefinitionSemanticSources(result *Result, id string) []string {
	lines := oracleDefinitionSemanticIDs[id]
	if len(lines) == 0 || !oracleRefusedConstruct(result) {
		return nil
	}
	return lines
}

func oracleRefusedConstruct(result *Result) bool {
	if result == nil || result.Ast == nil {
		return false
	}
	return oracleVisitGo(reflect.ValueOf(result.Ast), func(value any) bool {
		switch node := value.(type) {
		case *TypeDefinitionAst, *FunctionDefinitionAst:
			return true
		case *UsingStatementAst:
			return node.UsingStatementKind == UsingModule || node.UsingStatementKind == UsingAssembly
		}
		return false
	})
}

func compareOracle(goResult *Result, goErr error, reference oracleResult, schema oracleSchema) (oracleClass, string) {
	if reference.ReferenceException != "" {
		return oracleMismatch, "reference ParseInput exception: " + reference.ReferenceException
	}
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
	if goResult == nil || goResult.Ast == nil {
		return oracleMismatch, "Go returned no AST without an error"
	}
	if len(reference.Errors) != 0 {
		requireExact := false
		unresolvedEnum := false
		assemblyFilesystem := false
		for _, e := range reference.Errors {
			if e.ID == "UnexpectedAttribute" && oracleAttributedSemanticError(reference.Tree, e) {
				requireExact = true
			} else if e.ID == "DuplicateNamedArgument" && oracleDuplicateNamedArgument(reference.Tree, e) {
				requireExact = true
			} else if e.ID == "InvalidUnderlyingType" && oracleUnresolvedEnumError(reference.Tree, e) {
				requireExact, unresolvedEnum = true, true
			} else if e.ID == "ErrorLoadingAssembly" && oracleAssemblyFilesystemError(reference.Tree, e) {
				requireExact, assemblyFilesystem = true, true
			} else if len(oracleUsingModuleSemanticSources(goResult, reference.Tree, e)) != 0 {
				requireExact = true
			} else if !oracleSemanticIDs[e.ID] && len(oracleDefinitionSemanticSources(goResult, e.ID)) == 0 {
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
		if assemblyFilesystem {
			return oracleAssemblyFilesystem, fmt.Sprint(reference.Errors)
		}
		return oracleSemantic, fmt.Sprint(reference.Errors)
	}
	if detail := oracleCompareSuccess(goResult, reference, schema); detail != "" {
		return oracleMismatch, detail
	}
	return oracleMatch, ""
}

func oracleUsingModuleSemanticSources(result *Result, tree any, e oracleError) []string {
	lines := oracleSemanticEligibility.UsingModuleIDs[e.ID]
	if result == nil || len(lines) == 0 {
		return nil
	}
	for _, line := range strings.FieldsFunc(result.Text(Extent{0, len(result.src.text)}), func(r rune) bool { return r == '\r' || r == '\n' }) {
		line = strings.TrimSpace(line)
		if len(line) >= 9 && OrdinalEqualFold(line[:9], "#requires") {
			return nil
		}
	}
	if oracleWalk(tree, func(node map[string]any) bool {
		return node["Type"] == "UsingStatementAst" && node["UsingStatementKind"] == float64(UsingModule) && float64(e.Start) >= node["Start"].(float64) && float64(e.End) <= node["End"].(float64)
	}) {
		return lines
	}
	return nil
}

func oracleAssemblyFilesystemError(tree any, e oracleError) bool {
	return oracleWalk(tree, func(node map[string]any) bool {
		if node["Type"] != "UsingStatementAst" || node["UsingStatementKind"] != float64(UsingAssembly) {
			return false
		}
		name, ok := node["Name"].(map[string]any)
		if !ok || name["Type"] != "StringConstantExpressionAst" {
			return false
		}
		if name["Start"] != float64(e.Start) || name["End"] != float64(e.End) {
			return false
		}
		value, ok := oracleDecodeString(name["Value"])
		if !ok {
			return false
		}
		scheme, unc, absolute := absoluteURI(value)
		return absolute && !unc && scheme == "file"
	})
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

func TestOracleAssemblyFilesystemClass(t *testing.T) {
	o := startOracle(t)
	input := "using assembly /opt/sample-oracle.dll"
	result, err := Parse(input, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	response, err := o.batch([]string{input})
	if err != nil {
		t.Fatal(err)
	}
	reference := response.Results[0]
	using := findOracleNode(reference.Tree, "UsingStatementAst")
	name := using["Name"].(map[string]any)
	e := oracleError{ID: "ErrorLoadingAssembly", Start: int(name["Start"].(float64)), End: int(name["End"].(float64))}
	reference.Errors = []oracleError{e}
	if class, detail := compareOracle(result, nil, reference, o.schema); class != oracleAssemblyFilesystem {
		t.Fatalf("qualified assembly error: %s: %s", class, detail)
	}
	reference.Errors[0].Start++
	if class, _ := compareOracle(result, nil, reference, o.schema); class != oracleMismatch {
		t.Fatalf("unrelated error extent accepted: %s", class)
	}
	reference.Errors[0] = e
	token := reference.Tokens[0].(map[string]any)
	token["Flags"] = token["Flags"].(float64) + float64(FlagMemberName)
	if class, _ := compareOracle(result, nil, reference, o.schema); class != oracleMismatch {
		t.Fatalf("token mismatch accepted: %s", class)
	}
}

func TestOraclePrecedenceSchema(t *testing.T) {
	o := startOracle(t)
	response, err := o.batch([]string{"1 + 2 * 3 -eq 7"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(response.Version, "7.6.") {
		t.Skipf("precedence numeric pin is for 7.6, got %s", response.Version)
	}
	if !reflect.DeepEqual(response.Precedence, o.schema.Precedence) {
		t.Fatalf("raw runtime precedence does not match pinned Go values: runtime=%v Go=%v", response.Precedence, o.schema.Precedence)
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
		if t.Name() != "TestOracleCorpus" {
			path += "." + strings.ReplaceAll(t.Name(), "/", "_") + ".json"
		}
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

// This test is selected explicitly by the Windows workflow. The ordinary oracle
// remains the pinned 7 grammar; the second process checks the legacy meanings.
func TestOracleWindows51(t *testing.T) {
	if os.Getenv("LGRASS_PSPARSE_ORACLE_51") != "1" {
		t.Skip("oracle: Windows PowerShell 5.1 workflow only")
	}
	modern := startOracle(t)
	legacy := startOracleProgram(t, filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"))
	counts := make(map[string]int)
	report := make([]map[string]any, 0)
	metadata := make(map[string]any)
	complete := false
	stage := "reference startup"
	defer func() {
		if path := os.Getenv("LGRASS_PSPARSE_ORACLE_REPORT"); path != "" {
			data, err := json.MarshalIndent(map[string]any{"Complete": complete, "Stage": stage, "References": metadata, "Counts": counts, "Cases": report}, "", "  ")
			if err == nil {
				err = os.WriteFile(path, append(data, '\n'), 0644)
			}
			if err != nil {
				t.Errorf("oracle: write 5.1 report: %v", err)
			}
		}
	}()
	for _, check := range []struct {
		process *oracleProcess
		prefix  string
	}{{modern, "7.6.9"}, {legacy, "5.1."}} {
		response, err := check.process.batch([]string{""})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(response.Version, check.prefix) {
			t.Fatalf("oracle: required %s, got %s", check.prefix, response.Version)
		}
		t.Logf("oracle reference version %s", response.Version)
		t.Logf("reference %s binary precedence enum: %v", response.Version, response.Precedence)
		metadata[response.Version] = response.Precedence
	}
	stage = "corpus extraction"
	cases := append(loadOracleCorpus(t), loadOracleUpstream(t, modern)...)
	failures := 0
	firstErrorsEqual := 0
	for start := 0; start < len(cases); start += 16 {
		end := min(start+16, len(cases))
		inputs := make([]string, end-start)
		for i := range inputs {
			inputs[i] = cases[start+i].Input
		}
		stage = "7.6.9 batch starting at " + cases[start].Name
		newResponse, err := modern.batch(inputs)
		if err != nil {
			t.Fatal(err)
		}
		stage = "5.1 batch starting at " + cases[start].Name
		oldResponse, err := legacy.batch(inputs)
		if err != nil {
			t.Fatal(err)
		}
		for i, input := range inputs {
			stage = cases[start+i].Name
			class, detail, parseErr := compareOracleInput(input, newResponse.Results[i], modern.schema)
			if class == oracleMismatch || class == oracleUnreachable {
				t.Fatalf("7.6.9 prerequisite %s: %s", cases[start+i].Name, detail)
			}
			result, err := Parse(input, DefaultLimits)
			if (parseErr == nil) != (err == nil) {
				t.Fatal("oracle: non-deterministic parse")
			}
			oldClass, _ := compareOracle(result, err, oldResponse.Results[i], legacy.schema)
			label, oldDetail := oracleCompareWindows51(input, result, err, newResponse.Results[i], oldResponse.Results[i], modern.schema)
			counts[label]++
			if oldClass == oracleBothError {
				var parseError *ParseError
				if errors.As(err, &parseError) && parseError.ID == oldResponse.Results[i].Errors[0].ID && parseError.Extent.Start == oldResponse.Results[i].Errors[0].Start {
					firstErrorsEqual++
				}
			}
			entry := map[string]any{"Name": cases[start+i].Name, "Source": cases[start+i].Source, "Input": input, "Class": label, "Detail": oldDetail}
			for _, e := range oldResponse.Results[i].Errors {
				if !oracleSemanticIDs[e.ID] {
					lines := oracleDefinitionSemanticSources(result, e.ID)
					if len(lines) == 0 {
						lines = oracleUsingModuleSemanticSources(result, oldResponse.Results[i].Tree, e)
					}
					if len(lines) != 0 {
						if entry["DefinitionSemanticSources"] == nil {
							entry["DefinitionSemanticSources"] = map[string][]string{}
						}
						entry["DefinitionSemanticSources"].(map[string][]string)[e.ID] = lines
					}
				}
			}
			if label == string(oracleMismatch) || label == string(oracleUnreachable) {
				entry["ModernReference"] = newResponse.Results[i]
				entry["LegacyReference"] = oldResponse.Results[i]
			}
			report = append(report, entry)
			if label == string(oracleMismatch) || label == string(oracleUnreachable) {
				failures++
				if failures <= 20 {
					t.Errorf("5.1 %s (%s) input=%q: %s", cases[start+i].Name, cases[start+i].Source, input, oldDetail)
				}
			}
		}
	}
	t.Logf("5.1 oracle classes: %v; first error ID+offset agreement %d/%d; failures %d", counts, firstErrorsEqual, counts[string(oracleBothError)], failures)
	complete, stage = true, "finished"
	if failures > 20 {
		t.Errorf("%d additional 5.1 mismatches omitted", failures-20)
	}
}

func oracleCompareWindows51(input string, result *Result, err error, modern, legacy oracleResult, schema oracleSchema) (string, string) {
	class, detail := compareOracle(result, err, legacy, schema)
	if class != oracleMismatch {
		return string(class), detail
	}
	var e *ParseError
	if errors.As(err, &e) && len(modern.Errors) != 0 && modern.Errors[0].ID == e.ID && modern.Errors[0].Start == e.Extent.Start && modern.Errors[0].End == e.Extent.End {
		if legacy.ReferenceException != "" && oracleKnownLegacyException(input, e) {
			return "both-reject", "named 5.1 exception; Go and 7 report the same error"
		}
		if legacy.ReferenceException == "" && len(legacy.Errors) == 0 && oracleLegacySafeDeny(input, e, modern, legacy) {
			return "version-deny", "Go and 7 reject a checked legacy meaning"
		}
		return string(class), detail
	}
	if err != nil || legacy.ReferenceException != "" {
		return string(class), detail
	}
	if oracleBareUnicodeEscape(result) {
		return "refused-bare-escape", "judge refuses an unquoted Unicode escape with divergent 5.1 tokenization"
	}
	if len(legacy.Errors) != 0 && (oracleHasSevenFeature(result) || oracleSevenGrammarError(result, legacy.Errors[0])) {
		return "seven-only", "5.1 rejects a checked 7-only token or AST construct"
	}
	if len(legacy.Errors) == 0 {
		if diff, changed := oracleLegacyMeaningDiff(input, result, legacy, schema); diff == "" && changed {
			return "version-meaning", "checked clean, escape, numeric reading or default token classification"
		}
	}
	return string(class), detail
}

func oracleKnownLegacyException(input string, e *ParseError) bool {
	switch input {
	case "using namespace A = ,B", "using namespace x =\n", "using namespace x = \n", "using namespace x = ;", "using namespace x = ,", "using namespace x = &":
		return e.ID == "MissingNamespaceAlias" || e.ID == "UnexpectedUnaryOperator" || e.ID == "InvalidValueForUsingItemName"
	case "[int[,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,]]", "[int[,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,", "[int[,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,]", "[int[,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,]]":
		return e.ID == "ArrayHasTooManyDimensions"
	}
	return false
}

func oracleLegacySafeDeny(input string, e *ParseError, modern, legacy oracleResult) bool {
	switch e.ID {
	case "BadNumericConstant":
		return oracleSevenNumber(newSource(input).slice(e.Extent))
	case "InvalidUnicodeEscapeSequence", "MissingUnicodeEscapeSequenceTerminator", "TooManyDigitsInUnicodeEscapeSequence", "InvalidUnicodeEscapeSequenceValue":
		return strings.Contains(input, "`u")
	case "MissingNamedStatementBlock":
		if len(modern.Tokens) == 0 {
			return false
		}
		first, ok := modern.Tokens[0].(map[string]any)
		return ok && first["Kind"] == "Clean" && first["End"] == float64(e.Extent.Start) && e.Extent.Start == e.Extent.End
	case "InvalidValueForUsingItemName":
		tree, ok := legacy.Tree.(map[string]any)
		return ok && tree["Type"] == "ScriptBlockAst" && input == "using namespace A = $x" && !oracleWalk(legacy.Tree, func(node map[string]any) bool {
			kind, _ := node["Type"].(string)
			return kind != "ScriptBlockAst" && kind != "NamedBlockAst" && strings.HasSuffix(kind, "Ast")
		})
	}
	return false
}

func oracleBareUnicodeEscape(result *Result) bool {
	for _, token := range result.Tokens {
		if token.Kind != Generic && token.Kind != Identifier {
			continue
		}
		for i := 0; i+1 < len(token.Text); i++ {
			if token.Text[i] != '`' {
				continue
			}
			i++
			if strings.HasPrefix(token.Text[i:], "u{") {
				return true
			}
		}
	}
	return false
}

// These grammar differences are approved only at the error position belonging
// to the specific newer construct, never by error ID alone.
func oracleSevenGrammarError(result *Result, e oracleError) bool {
	for _, token := range result.Tokens {
		if e.ID == "TerminatorExpectedAtEndOfString" && (token.Kind == HereStringExpandable || token.Kind == HereStringLiteral) && e.Start >= token.Extent.Start && e.End <= token.Extent.End {
			return true
		}
		if e.ID == "OpenBraceNeedsToBeBackTickedInVariableName" && token.Kind == Variable && strings.Contains(token.Text, "`u{") && e.Start >= token.Extent.Start && e.End <= token.Extent.End {
			return true
		}
	}
	return oracleVisitGo(reflect.ValueOf(result.Ast), func(value any) bool {
		switch node := value.(type) {
		case *TypeDefinitionAst:
			if e.ID == "MissingTypeBody" && node.TypeAttributes&TypeAttributesEnum != 0 && len(node.BaseTypes) != 0 {
				for i, token := range result.Tokens {
					if token.Kind == Enum && token.Extent.Start == node.Extent().Start && i+1 < len(result.Tokens) {
						name := result.Tokens[i+1]
						return e.Start == name.Extent.End && e.End == e.Start
					}
				}
			}
			if e.ID == "MissingEndCurlyBrace" && node.TypeAttributes&TypeAttributesClass != 0 {
				open := -1
				for _, token := range result.Tokens {
					if token.Kind == LCurly && token.Extent.Start >= node.Extent().Start && token.Extent.End <= node.Extent().End {
						if e.Start == token.Extent.Start && e.End == token.Extent.End {
							open = e.Start
						}
						break
					}
				}
				if e.Start != open {
					return false
				}
				for _, member := range node.Members {
					method, ok := member.(*FunctionMemberAst)
					if !ok || method.Body == nil {
						continue
					}
					for _, token := range result.Tokens {
						if token.Extent.Start >= method.Extent().Start && token.Extent.End <= method.Body.Extent().Start && token.Kind.Traits()&FlagKeyword != 0 && OrdinalEqualFold(token.Text, method.Name) {
							return true
						}
					}
				}
			}
		case *GenericTypeName:
			if e.ID == "UnexpectedToken" {
				for _, argument := range node.GenericArguments {
					if array, ok := argument.(*ArrayTypeName); ok {
						raw := result.Text(array.Extent())
						if !strings.HasPrefix(raw, "[") && oracleVisitGo(reflect.ValueOf(result.Ast), func(value any) bool {
							literal, ok := value.(*TypeExpressionAst)
							return ok && literal.Extent().Start <= node.Extent().Start && literal.Extent().End >= node.Extent().End && ((e.Start == literal.Extent().End-1 && e.End == literal.Extent().End) || (e.Start == node.Extent().End-1 && e.End == node.Extent().End))
						}) {
							return true
						}
					}
				}
			}
		case *PipelineAst:
			if e.ID == "EmptyPipeElement" && node.Extent().Start < e.Start && node.Extent().End > e.End {
				for i, token := range result.Tokens {
					if token.Kind == Pipe && token.Extent == (Extent{e.Start, e.End}) {
						for j := i - 1; j >= 0; j-- {
							previous := result.Tokens[j]
							if previous.Kind == NewLine {
								return true
							}
							if previous.Kind != Comment {
								break
							}
						}
					}
				}
			}
		case *HashtableAst:
			if e.ID == "IncompleteHashLiteral" {
				for i, pair := range node.KeyValuePairs {
					condition, ok := pair.Value.(*IfStatementAst)
					if ok && condition.ElseClause == nil && i+1 < len(node.KeyValuePairs) && e.Start == condition.Extent().End && e.End == e.Start && strings.ContainsAny(result.Text(Extent{condition.Extent().End, node.KeyValuePairs[i+1].Key.Extent().Start}), "\r\n") {
						return true
					}
				}
			}
		case *InvokeMemberExpressionAst:
			if e.ID == "MissingEndSquareBracket" {
				for _, arg := range node.Arguments {
					if oracleVisitGo(reflect.ValueOf(arg), func(value any) bool {
						index, ok := value.(*IndexExpressionAst)
						if !ok {
							return false
						}
						array, ok := index.Index.(*ArrayLiteralAst)
						return ok && len(array.Elements) > 1 && e.Start == array.Elements[0].Extent().End && e.End == e.Start
					}) {
						return true
					}
				}
			}
		}
		return false
	})
}

func oracleVisitGo(value reflect.Value, match func(any) bool) bool {
	if !value.IsValid() {
		return false
	}
	if value.CanInterface() && match(value.Interface()) {
		return true
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		return !value.IsNil() && oracleVisitGo(value.Elem(), match)
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if oracleVisitGo(value.Field(i), match) {
				return true
			}
		}
	case reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			if oracleVisitGo(value.Index(i), match) {
				return true
			}
		}
	}
	return false
}

func oracleHasSevenFeature(result *Result) bool {
	for _, token := range result.Tokens {
		switch token.Kind {
		case AndAnd, OrOr, QuestionMark, QuestionQuestion, QuestionQuestionEquals, QuestionDot, QuestionLBracket:
			return true
		case Number:
			if strings.HasPrefix(strings.TrimLeft(strings.ToLower(token.Text), "+-"), "0b") {
				return true
			}
			switch token.Value.(type) {
			case int8, uint8, int16, uint16, uint32, uint64, *big.Int:
				return true
			}
		}
	}
	tree := oracleJSON(oracleValue(reflect.ValueOf(result.Ast), oracleSpec{Kind: "node"}, makeOracleSchema()))
	return oracleWalk(tree, func(node map[string]any) bool {
		if node["Type"] == "NamedBlockAst" && node["BlockKind"] == float64(Clean) {
			return true
		}
		if node["Background"] == true || node["NullConditional"] == true {
			return true
		}
		arguments, ok := node["GenericTypeArguments"].([]any)
		return ok && len(arguments) > 0
	})
}

func oracleJSON(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	canonicalOracleNumbers(result)
	return result
}

func oracleSevenNumber(text string) bool {
	return strings.HasPrefix(strings.TrimLeft(strings.ToLower(text), "+-"), "0b") || strings.ContainsAny(text, "uUyYsSnN")
}

func oracleLegacyNumericRole(tree any, start, end int) (TokenFlags, bool) {
	var flags TokenFlags
	ok := oracleWalk(tree, func(node map[string]any) bool {
		matches := func(value any) bool {
			constant, ok := value.(map[string]any)
			return ok && constant["Type"] == "ConstantExpressionAst" && constant["Start"] == float64(start) && constant["End"] == float64(end)
		}
		if node["Type"] == "CommandExpressionAst" && matches(node["Expression"]) {
			flags = FlagCommandName
			return true
		}
		if node["Type"] == "CommandAst" {
			if elements, ok := node["CommandElements"].([]any); ok {
				for _, element := range elements {
					if matches(element) {
						flags = FlagNone
						return true
					}
				}
			}
		}
		return false
	})
	return flags, ok
}

// Only command positions gain the legacy generic-word reading. Constants in
// expressions remain numbers, since 5.1 rejects those newer spellings there.
func oracleLegacyNumericTree(node map[string]any, src source) map[string]any {
	word := func(value any) map[string]any {
		constant, ok := value.(map[string]any)
		if !ok || constant["Type"] != "ConstantExpressionAst" {
			return nil
		}
		start, end := int(constant["Start"].(float64)), int(constant["End"].(float64))
		raw := src.slice(Extent{start, end})
		if !oracleSevenNumber(raw) {
			return nil
		}
		return map[string]any{"Type": "StringConstantExpressionAst", "Start": float64(start), "End": float64(end), "Value": oracleJSON(oracleString(raw)), "StringConstantType": float64(BareWord)}
	}
	if node["Type"] == "CommandExpressionAst" {
		if name := word(node["Expression"]); name != nil {
			return map[string]any{"Type": "CommandAst", "Start": node["Start"], "End": node["End"], "CommandElements": []any{name}, "InvocationOperator": float64(Unknown), "Redirections": node["Redirections"]}
		}
	}
	if node["Type"] == "CommandAst" {
		if elements, ok := node["CommandElements"].([]any); ok {
			copyElements := append([]any(nil), elements...)
			changed := false
			for i, element := range elements {
				if name := word(element); name != nil {
					copyElements[i], changed = name, true
				}
			}
			if changed {
				copyNode := make(map[string]any, len(node))
				for key, value := range node {
					copyNode[key] = value
				}
				copyNode["CommandElements"] = copyElements
				return copyNode
			}
		}
	}
	return nil
}

func oracleLegacyMeaningDiff(input string, result *Result, reference oracleResult, schema oracleSchema) (string, bool) {
	actual := oracleJSON(oracleValue(reflect.ValueOf(result.Ast), oracleSpec{Kind: "node"}, schema))
	reference.Tree = oracleJSON(reference.Tree)
	changed := false
	if diff := oracleLegacyTreeDiff(actual, reference.Tree, newSource(input), "$", &changed); diff != "" {
		return diff, false
	}
	tokens := make([]any, 0, len(result.Tokens))
	for _, token := range result.Tokens {
		tokens = append(tokens, oracleToken(token))
	}
	a := oracleJSON(tokens).([]any)
	if len(a) != len(reference.Tokens) {
		return "token count differs", false
	}
	for i := range a {
		x := a[i].(map[string]any)
		y := reference.Tokens[i].(map[string]any)
		if (x["Kind"] == "Default" || x["Kind"] == "Generic") && (y["Kind"] == "Default" || y["Kind"] == "Identifier") {
			flags, ok := x["Flags"].(float64)
			start, end := int(x["Start"].(float64)), int(x["End"].(float64))
			if ok && (x["Kind"] != y["Kind"] || x["Flags"] != y["Flags"]) && y["Flags"] == float64(uint32(flags)&^uint32(FlagKeyword)) && OrdinalEqualFold(newSource(input).slice(Extent{start, end}), "default") {
				x["Flags"], x["Kind"] = y["Flags"], y["Kind"]
				changed = true
			}
		}
		if changed && x["Kind"] == "Clean" && y["Kind"] == "Identifier" && x["Class"] == "Token" && y["Class"] == "Token" && x["Flags"] == float64(Clean.Traits()) && y["Flags"] == float64(FlagCommandName) && x["Start"] == y["Start"] && x["End"] == y["End"] {
			start, end := int(x["Start"].(float64)), int(x["End"].(float64))
			if OrdinalEqualFold(newSource(input).slice(Extent{start, end}), "clean") {
				x["Kind"], x["Flags"] = y["Kind"], y["Flags"]
			}
		}
		if x["Kind"] == "Number" && y["Kind"] == "Generic" && x["Class"] == "NumberToken" && y["Class"] == "StringLiteralToken" {
			start, end := int(x["Start"].(float64)), int(x["End"].(float64))
			if flags, ok := oracleLegacyNumericRole(actual, start, end); ok && oracleSevenNumber(newSource(input).slice(Extent{start, end})) && y["Flags"] == float64(flags) {
				x["Kind"], x["Class"], x["Flags"] = y["Kind"], y["Class"], y["Flags"]
				changed = true
			}
		}
		if y["Kind"] == "Generic" && x["Class"] == "Token" && y["Class"] == "StringLiteralToken" && x["Start"] == y["Start"] && x["End"] == y["End"] {
			start, end := int(x["Start"].(float64)), int(x["End"].(float64))
			named := changed && x["Kind"] == "Clean" && x["Flags"] == float64(Clean.Traits()) && y["Flags"] == float64(FlagCommandName)
			literal := oracleWalk(actual, func(node map[string]any) bool {
				if node["Type"] != "StringConstantExpressionAst" || node["StringConstantType"] != float64(BareWord) || node["Start"] != float64(start) || node["End"] != float64(end) {
					return false
				}
				value, ok := oracleDecodeString(node["Value"])
				return ok && OrdinalEqualFold(value, "clean")
			})
			commandName := literal && x["Kind"] == "Clean" && x["Flags"] == float64(FlagKeyword|FlagCommandName) && y["Flags"] == float64(FlagCommandName)
			argument := literal && x["Kind"] == "Generic" && x["Flags"] == float64(FlagKeyword) && y["Flags"] == float64(FlagNone)
			if (named || commandName || argument) && OrdinalEqualFold(newSource(input).slice(Extent{start, end}), "clean") {
				changed = true
				continue
			}
		}
		if diff := oracleFirstDiff(x, y, fmt.Sprintf("tokens[%d]", i)); diff != "" {
			return diff, false
		}
	}
	return "", changed
}

func oracleLegacyTreeDiff(actual, reference any, src source, path string, changed *bool) string {
	if reflect.DeepEqual(actual, reference) {
		return ""
	}
	a, aok := actual.(map[string]any)
	b, bok := reference.(map[string]any)
	if aok && bok {
		if a["Type"] == "AssignmentStatementAst" && b["Type"] == "AssignmentStatementAst" {
			if right, ok := a["Right"].(map[string]any); ok && right["Type"] == "CommandExpressionAst" {
				if command := oracleLegacyNumericTree(right, src); command != nil {
					copyNode := make(map[string]any, len(a))
					for key, value := range a {
						copyNode[key] = value
					}
					copyNode["Right"] = map[string]any{"Type": "PipelineAst", "Start": right["Start"], "End": right["End"], "Background": false, "PipelineElements": []any{command}}
					a = copyNode
					*changed = true
				}
			}
		}
		if a["Type"] == "Double" && b["Type"] == "Double" && a["Value"] == "-0" && b["Value"] == "0" && len(a) == 2 && len(b) == 2 {
			*changed = true
			return ""
		}
		if replacement := oracleLegacyNumericTree(a, src); replacement != nil {
			a = replacement
			*changed = true
		}
		if a["Type"] == "ScriptBlockAst" && b["Type"] == "ScriptBlockAst" && a["CleanBlock"] != nil && b["CleanBlock"] == nil {
			if replacement, ok := oracleLegacyClean(a, b); ok {
				reference, b = replacement, replacement
				*changed = true
			}
		}
		if len(a) != len(b) {
			return oracleFirstDiff(a, b, path)
		}
		for key, x := range a {
			y, ok := b[key]
			if !ok {
				return path + "." + key + " missing"
			}
			if (key == "Value" || key == "FormatString") && (a["Type"] == "StringConstantExpressionAst" || a["Type"] == "ExpandableStringExpressionAst") && a["Type"] == b["Type"] && a["Start"] == b["Start"] && a["End"] == b["End"] && (a["StringConstantType"] == float64(DoubleQuoted) || a["StringConstantType"] == float64(DoubleQuotedHereString)) {
				newValue, newOK := oracleDecodeString(x)
				oldValue, oldOK := oracleDecodeString(y)
				if newOK && oldOK && newValue != oldValue {
					raw := src.slice(Extent{int(a["Start"].(float64)), int(a["End"].(float64))})
					if oracleLegacyEscapesMatch(newValue, oldValue, raw) {
						*changed = true
						continue
					}
				}
			}
			if diff := oracleLegacyTreeDiff(x, y, src, path+"."+key, changed); diff != "" {
				return diff
			}
		}
		return ""
	}
	if a, aok := actual.([]any); aok {
		if b, bok := reference.([]any); bok && len(a) == len(b) {
			for i := range a {
				if diff := oracleLegacyTreeDiff(a[i], b[i], src, fmt.Sprintf("%s[%d]", path, i), changed); diff != "" {
					return diff
				}
			}
			return ""
		}
	}
	return oracleFirstDiff(actual, reference, path)
}

// A legacy clean command must consist of exactly the literal name and its
// script-block argument. Preserve and compare the body's actual statements.
func oracleLegacyClean(modern, legacy map[string]any) (map[string]any, bool) {
	clean, ok := modern["CleanBlock"].(map[string]any)
	if !ok {
		return nil, false
	}
	end, ok := legacy["EndBlock"].(map[string]any)
	if !ok {
		return nil, false
	}
	statements, ok := end["Statements"].([]any)
	if !ok || len(statements) != 1 {
		return nil, false
	}
	pipeline, ok := statements[0].(map[string]any)
	if !ok || pipeline["Type"] != "PipelineAst" {
		return nil, false
	}
	elements, ok := pipeline["PipelineElements"].([]any)
	if !ok || len(elements) != 1 {
		return nil, false
	}
	command, ok := elements[0].(map[string]any)
	if !ok || command["Type"] != "CommandAst" || command["Start"] != clean["Start"] || command["End"] != clean["End"] {
		return nil, false
	}
	if command["InvocationOperator"] != float64(Unknown) {
		return nil, false
	}
	if redirects, ok := command["Redirections"].([]any); !ok || len(redirects) != 0 {
		return nil, false
	}
	arguments, ok := command["CommandElements"].([]any)
	if !ok || len(arguments) != 2 {
		return nil, false
	}
	name, ok := arguments[0].(map[string]any)
	if !ok {
		return nil, false
	}
	text, ok := oracleDecodeString(name["Value"])
	if !ok || !OrdinalEqualFold(text, "clean") {
		return nil, false
	}
	block, ok := arguments[1].(map[string]any)
	if !ok || block["Type"] != "ScriptBlockExpressionAst" {
		return nil, false
	}
	body, ok := block["ScriptBlock"].(map[string]any)
	if !ok {
		return nil, false
	}
	for _, field := range []string{"Attributes", "UsingStatements"} {
		if values, ok := body[field].([]any); !ok || len(values) != 0 {
			return nil, false
		}
	}
	if body["ScriptRequirements"] != nil {
		return nil, false
	}
	for _, field := range []string{"BeginBlock", "ProcessBlock", "CleanBlock", "DynamicParamBlock", "ParamBlock"} {
		if body[field] != nil {
			return nil, false
		}
	}
	bodyEnd, ok := body["EndBlock"].(map[string]any)
	if !ok {
		return nil, false
	}
	newClean := make(map[string]any)
	for key, value := range clean {
		newClean[key] = value
	}
	newClean["Statements"], newClean["Traps"] = bodyEnd["Statements"], bodyEnd["Traps"]
	replacement := make(map[string]any)
	for key, value := range legacy {
		replacement[key] = value
	}
	replacement["CleanBlock"], replacement["EndBlock"] = newClean, modern["EndBlock"]
	return replacement, true
}

type oracleEscapeEdit struct{ modern, legacy string }

func oracleLegacyEscapesMatch(modern, legacy, raw string) bool {
	var edits []oracleEscapeEdit
	for i := 0; i+1 < len(raw); i++ {
		if raw[i] != '`' {
			continue
		}
		i++
		if raw[i] == 'e' {
			edits = append(edits, oracleEscapeEdit{"\x1b", "e"})
			continue
		}
		if raw[i] != 'u' || i+1 >= len(raw) || raw[i+1] != '{' {
			continue
		}
		end := strings.IndexByte(raw[i+2:], '}')
		if end < 0 {
			continue
		}
		end += i + 2
		code, err := strconv.ParseUint(raw[i+2:end], 16, 32)
		if err != nil || code > 0x10ffff {
			continue
		}
		text := string(rune(code))
		if code >= 0xd800 && code <= 0xdfff {
			text = utf16ToString([]uint16{uint16(code)})
		}
		edits = append(edits, oracleEscapeEdit{text, raw[i : end+1]})
		i = end
	}
	if len(edits) == 0 || len(edits) > 32 {
		return false
	}
	type position struct{ modern, legacy, edit int }
	seen := make(map[position]bool)
	var match func(int, int, int) bool
	match = func(mi, li, ei int) bool {
		if mi == len(modern) && li == len(legacy) {
			return true
		}
		p := position{mi, li, ei}
		if seen[p] || len(seen) > 100000 {
			return false
		}
		seen[p] = true
		if mi < len(modern) && li < len(legacy) && modern[mi] == legacy[li] && match(mi+1, li+1, ei) {
			return true
		}
		for k := ei; k < len(edits); k++ {
			edit := edits[k]
			if strings.HasPrefix(modern[mi:], edit.modern) && strings.HasPrefix(legacy[li:], edit.legacy) && match(mi+len(edit.modern), li+len(edit.legacy), k+1) {
				return true
			}
		}
		return false
	}
	return match(0, 0, 0)
}

func TestOracleLegacyMeanings(t *testing.T) {
	o := startOracle(t)
	t.Run("clean shape", func(t *testing.T) {
		input := "clean { Get-Item foo }"
		result, err := Parse(input, DefaultLimits)
		if err != nil {
			t.Fatal(err)
		}
		response, err := o.batch([]string{"& " + input})
		if err != nil {
			t.Fatal(err)
		}
		legacy := response.Results[0]
		shift := func(node map[string]any) bool {
			for _, key := range []string{"Start", "End"} {
				if value, ok := node[key].(float64); ok {
					node[key] = max(0, value-2)
				}
			}
			return false
		}
		oracleWalk(legacy.Tree, shift)
		legacy.Tokens = legacy.Tokens[1:]
		for _, token := range legacy.Tokens {
			oracleWalk(token, shift)
		}
		first := legacy.Tokens[0].(map[string]any)
		first["Kind"], first["Class"], first["Flags"] = "Generic", "StringLiteralToken", float64(FlagCommandName)
		command := findOracleNode(legacy.Tree, "CommandAst")
		command["InvocationOperator"] = float64(Unknown)
		if diff, changed := oracleLegacyMeaningDiff(input, result, legacy, o.schema); diff != "" || !changed {
			t.Fatalf("clean reading rejected, changed=%t: %s", changed, diff)
		}
		block := findOracleNode(command, "ScriptBlockExpressionAst")
		body := findOracleNode(block, "CommandAst")
		elements := body["CommandElements"].([]any)
		elements[0].(map[string]any)["Value"] = oracleJSON(oracleString("Get-Content"))
		if diff, _ := oracleLegacyMeaningDiff(input, result, legacy, o.schema); diff == "" {
			t.Fatal("changed clean body accepted")
		}
	})
	t.Run("escape fields", func(t *testing.T) {
		input := "\"`e `u{41} $x\""
		result, err := Parse(input, DefaultLimits)
		if err != nil {
			t.Fatal(err)
		}
		response, err := o.batch([]string{input})
		if err != nil {
			t.Fatal(err)
		}
		legacy := response.Results[0]
		node := findOracleNode(legacy.Tree, "ExpandableStringExpressionAst")
		node["Value"] = oracleJSON(oracleString("e u{41} $x"))
		node["FormatString"] = oracleJSON(oracleString("e u{41} {0}"))
		if diff, changed := oracleLegacyMeaningDiff(input, result, legacy, o.schema); diff != "" || !changed {
			t.Fatalf("escape reading rejected, changed=%t: %s", changed, diff)
		}
		node["Value"] = oracleJSON(oracleString("e u{42} $x"))
		if diff, _ := oracleLegacyMeaningDiff(input, result, legacy, o.schema); diff == "" {
			t.Fatal("unrelated escape difference accepted")
		}
	})
	for _, test := range []struct {
		modern, legacy, raw string
		want                bool
	}{
		{"\x1b", "e", "\"`e\"", true}, {"u", "u{75}", "\"`u{75}\"", true},
		{"A", "u{41}", "\"`u{41}\"", true}, {"A", "u{42}", "\"`u{41}\"", false},
		{"\x1b", "e", "\"``e\"", false}, {"x", "y", "\"`e\"", false},
	} {
		if got := oracleLegacyEscapesMatch(test.modern, test.legacy, test.raw); got != test.want {
			t.Errorf("legacy edit %q -> %q in %q: got %t", test.modern, test.legacy, test.raw, got)
		}
	}
}

func TestOracleSevenFeatures(t *testing.T) {
	for _, test := range []struct {
		input string
		want  bool
	}{
		{"Get-Item foo", false}, {"$x[0]", false}, {"& clean {}", false},
		{"a && b", true}, {"a || b", true}, {"$x ? 1 : 2", true}, {"$x ?? 1", true},
		{"$x ??= 1", true}, {"${x}?.Name", true}, {"${x}?[0]", true}, {"a &", true},
		{"clean { a }", true}, {"0b101", true}, {"1u", true}, {"1n", true},
	} {
		result, err := Parse(test.input, DefaultLimits)
		if err != nil {
			t.Fatalf("%q: %v", test.input, err)
		}
		if got := oracleHasSevenFeature(result); got != test.want {
			t.Errorf("7-only feature %q: got %t want %t", test.input, got, test.want)
		}
	}
}

func TestOracleLegacyGrammarShapes(t *testing.T) {
	for _, test := range []struct {
		input, id  string
		start, end int
	}{
		{"enum E : byte { A }", "MissingTypeBody", 6, 6},
		{"class C { if() {} }", "MissingEndCurlyBrace", 8, 9},
		{"class C { foreach() {} }", "MissingEndCurlyBrace", 8, 9},
		{"[System.Tuple[System.String[],System.Int32[]]]", "UnexpectedToken", 45, 46},
		{"[System.Collections.Generic.Dictionary[int[], System.Collections.Generic.List[string[]]]]", "UnexpectedToken", 87, 88},
	} {
		t.Run(test.input, func(t *testing.T) {
			result, err := Parse(test.input, DefaultLimits)
			if err != nil {
				t.Fatal(err)
			}
			e := oracleError{ID: test.id, Start: test.start, End: test.end}
			if !oracleSevenGrammarError(result, e) {
				t.Fatalf("approved shape rejected: %+v; tokens=%+v", e, result.Tokens)
			}
			e.Start += 10
			e.End += 10
			if oracleSevenGrammarError(result, e) {
				t.Fatal("wrong error extent accepted")
			}
			e.Start -= 10
			e.End -= 10
			e.ID = "UnexpectedAttribute"
			if oracleSevenGrammarError(result, e) {
				t.Fatal("wrong error ID accepted")
			}
		})
	}
	for _, input := range []string{"enum E { A }", "class C { f() {} }", "[System.Collections.Generic.List[string]]"} {
		result, err := Parse(input, DefaultLimits)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"MissingTypeBody", "MissingEndCurlyBrace", "UnexpectedToken"} {
			if oracleSevenGrammarError(result, oracleError{ID: id}) {
				t.Fatalf("unqualified input accepted: %s", input)
			}
		}
	}
}

func TestOracleReferenceExceptionFails(t *testing.T) {
	for _, input := range []string{"Get-Item foo", "("} {
		result, err := Parse(input, DefaultLimits)
		class, _ := compareOracle(result, err, oracleResult{ReferenceException: "Unrecoverable error in Windows PowerShell."}, makeOracleSchema())
		if class != oracleMismatch {
			t.Fatalf("reference exception class = %s", class)
		}
	}
}

func TestOracleDefinitionSemanticQualification(t *testing.T) {
	definition, err := Parse("class C { [int] M() {} }", DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Parse("Get-Item foo", DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if lines := oracleDefinitionSemanticSources(definition, "MethodHasCodePathNotReturn"); !reflect.DeepEqual(lines, []string{"SemanticChecks.cs:427"}) {
		t.Fatalf("source evidence = %v", lines)
	}
	if len(oracleDefinitionSemanticSources(plain, "MethodHasCodePathNotReturn")) != 0 {
		t.Fatal("definition-only ID accepted on plain command")
	}
	for _, id := range []string{"UnexpectedAttribute", "DuplicateNamedArgument", "MissingEndCurlyBrace", "UnknownID"} {
		if len(oracleDefinitionSemanticSources(definition, id)) != 0 {
			t.Fatalf("syntax/shared/unknown ID eligible: %s", id)
		}
	}
	module, err := Parse("using module 0*", DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if lines := oracleDefinitionSemanticSources(module, "WildCardModuleNameError"); !reflect.DeepEqual(lines, []string{"SymbolResolver.cs:559"}) {
		t.Fatalf("module source evidence = %v", lines)
	}
	namespace, err := Parse("using namespace System", DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if len(oracleDefinitionSemanticSources(namespace, "WildCardModuleNameError")) != 0 {
		t.Fatal("namespace using qualified for module semantic allowance")
	}
}

func TestOracleUsingModuleSharedSemantic(t *testing.T) {
	o := startOracle(t)
	for _, test := range []struct{ input, id, line string }{
		{"using module .", "RequiresModuleInvalid", "SymbolResolver.cs:552"},
		{"#requires -version 1.0\nusing module .", "RequiresModuleInvalid", "SymbolResolver.cs:552"},
		{"using module 00A = @{A00000000000=A}", "RequiresArgumentMustBeConstant", "SymbolResolver.cs:545"},
		{"#requires -version 1.0\nusing module 00A = @{A00000000000=A}", "RequiresArgumentMustBeConstant", "SymbolResolver.cs:545"},
	} {
		input := test.input
		result, err := Parse(input, DefaultLimits)
		if err != nil {
			t.Fatal(err)
		}
		response, err := o.batch([]string{input})
		if err != nil {
			t.Fatal(err)
		}
		reference := response.Results[0]
		var observed *oracleError
		for i := range reference.Errors {
			if reference.Errors[i].ID == test.id {
				observed = &reference.Errors[i]
				break
			}
		}
		if observed == nil {
			t.Fatalf("expected shared semantic ID: %v", reference.Errors)
		}
		lines := oracleUsingModuleSemanticSources(result, reference.Tree, *observed)
		if strings.HasPrefix(input, "#requires") {
			if len(lines) != 0 {
				t.Fatal("shared ID waived despite #requires line")
			}
			continue
		}
		if !reflect.DeepEqual(lines, []string{test.line}) {
			t.Fatalf("source evidence=%v", lines)
		}
		if class, detail := compareOracle(result, nil, reference, o.schema); class != oracleSemantic {
			t.Fatalf("module semantic rejected: %s %s", class, detail)
		}
		wrong := *observed
		wrong.Start = -1
		if len(oracleUsingModuleSemanticSources(result, reference.Tree, wrong)) != 0 {
			t.Fatal("wrong error extent accepted")
		}
		using := findOracleNode(reference.Tree, "UsingStatementAst")
		using["UsingStatementKind"] = float64(UsingNamespace)
		if len(oracleUsingModuleSemanticSources(result, reference.Tree, *observed)) != 0 {
			t.Fatal("namespace accepted by module shape")
		}
	}
}

// Replaying a captured failing report exercises exact Windows evidence on the
// host while the actual 5.1 verification remains the workflow's responsibility.
func TestOracleWindows51Replay(t *testing.T) {
	path := os.Getenv("LGRASS_PSPARSE_ORACLE_REPLAY")
	if path == "" {
		path = filepath.Join("oracle", "windows51-regressions.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct {
		Name, Input                      string
		ModernReference, LegacyReference oracleResult
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	schema := makeOracleSchema()
	for _, entry := range entries {
		result, err := Parse(entry.Input, DefaultLimits)
		label, detail := oracleCompareWindows51(entry.Input, result, err, entry.ModernReference, entry.LegacyReference, schema)
		if label == string(oracleMismatch) || label == string(oracleUnreachable) {
			if result != nil && len(entry.LegacyReference.Errors) == 0 {
				diff, _ := oracleLegacyMeaningDiff(entry.Input, result, entry.LegacyReference, schema)
				detail += "; legacy qualification: " + diff
			}
			t.Errorf("%s input=%q: %s", entry.Name, entry.Input, detail)
		}
	}
}

func TestOracleDefaultQualification(t *testing.T) {
	o := startOracle(t)
	for _, input := range []string{"switch ($x) { default { 1 } }", "ſwitch ($x) { default { 1 } }"} {
		result, err := Parse(input, DefaultLimits)
		if err != nil {
			t.Fatal(err)
		}
		response, err := o.batch([]string{input})
		if err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"Default", "Identifier"} {
			if strings.HasPrefix(input, "ſ") && kind == "Default" {
				continue
			}
			legacy := response.Results[0]
			legacy.Tokens = oracleJSON(legacy.Tokens).([]any)
			for _, raw := range legacy.Tokens {
				token := raw.(map[string]any)
				if token["Kind"] == "Default" {
					token["Kind"] = kind
					token["Flags"] = float64(uint32(token["Flags"].(float64)) &^ uint32(FlagKeyword))
				}
			}
			if diff, changed := oracleLegacyMeaningDiff(input, result, legacy, o.schema); diff != "" || !changed {
				t.Fatalf("default reading rejected for %q as %s: changed=%v diff=%s tokens=%v", input, kind, changed, diff, legacy.Tokens)
			}
			legacy.Tokens[0].(map[string]any)["End"] = float64(999)
			if diff, _ := oracleLegacyMeaningDiff(input, result, legacy, o.schema); diff == "" {
				t.Fatal("unrelated token change accepted")
			}
		}
	}
}

func TestOracleNumericQualification(t *testing.T) {
	o := startOracle(t)
	for _, input := range []string{"0b1", "Write-Output 10u"} {
		result, err := Parse(input, DefaultLimits)
		if err != nil {
			t.Fatal(err)
		}
		response, err := o.batch([]string{input})
		if err != nil {
			t.Fatal(err)
		}
		legacy := response.Results[0]
		var numeric *Token
		for _, token := range result.Tokens {
			if token.Kind == Number {
				numeric = token
				break
			}
		}
		word := oracleJSON(oracleValue(reflect.ValueOf(newStringConstantExpressionAst(numeric.Extent, numeric.Text, BareWord)), oracleSpec{Kind: "node"}, o.schema))
		command := findOracleNode(legacy.Tree, "CommandAst")
		flags := FlagNone
		if command == nil {
			command = findOracleNode(legacy.Tree, "CommandExpressionAst")
			delete(command, "Expression")
			command["Type"], command["CommandElements"], command["InvocationOperator"] = "CommandAst", []any{word}, float64(Unknown)
			flags = FlagCommandName
		} else {
			command["CommandElements"].([]any)[1] = word
		}
		for _, raw := range legacy.Tokens {
			token := raw.(map[string]any)
			if token["Kind"] == "Number" {
				token["Kind"], token["Class"], token["Flags"] = "Generic", "StringLiteralToken", float64(flags)
			}
		}
		if diff, changed := oracleLegacyMeaningDiff(input, result, legacy, o.schema); diff != "" || !changed {
			t.Fatalf("numeric reading rejected: %s", diff)
		}
		word.(map[string]any)["Value"] = oracleJSON(oracleString("wrong-command"))
		if diff, _ := oracleLegacyMeaningDiff(input, result, legacy, o.schema); diff == "" {
			t.Fatal("wrong numeric command/argument accepted")
		}
		word.(map[string]any)["Value"] = oracleJSON(oracleString(numeric.Text))
		for _, raw := range legacy.Tokens {
			token := raw.(map[string]any)
			if token["Start"] == float64(numeric.Extent.Start) {
				token["Flags"] = float64(FlagKeyword)
			}
		}
		if diff, _ := oracleLegacyMeaningDiff(input, result, legacy, o.schema); diff == "" {
			t.Fatal("wrong numeric rescan flags accepted")
		}
	}
}
