// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// The node types mirror ast.cs: a Go struct per concrete Ast class with the same name, and a field per public
// property with the same name, so the oracle compares trees by type and field name. Abstract classes are
// interfaces. Left out on purpose: Parent (the oracle skips it too), StaticType and anything else computed by
// type inference, the error recovery nodes (parsing stops at the first error), and DSC configuration and dynamic
// keyword nodes (reported as unsupported).

// Ast is any syntax tree node.
type Ast interface {
	Extent() Extent
}

// StatementAst is any statement.
type StatementAst interface {
	Ast
	isStatement()
}

// PipelineBaseAst is a pipeline, a pipeline chain or an assignment.
type PipelineBaseAst interface {
	StatementAst
	isPipelineBase()
}

// ChainableAst is a pipeline or a pipeline chain.
type ChainableAst interface {
	PipelineBaseAst
	isChainable()
}

// CommandBaseAst is one element of a pipeline: a command or an expression.
type CommandBaseAst interface {
	StatementAst
	GetRedirections() []RedirectionAst
}

// CommandElementAst is a command name, parameter or argument.
type CommandElementAst interface {
	Ast
	isCommandElement()
}

// ExpressionAst is any expression.
type ExpressionAst interface {
	CommandElementAst
	isExpression()
}

// RedirectionAst is a file or merging redirection.
type RedirectionAst interface {
	Ast
	GetFromStream() RedirectionStream
}

// MemberAst is a class or enum member.
type MemberAst interface {
	Ast
	GetName() string
}

// AttributeBaseAst is an attribute or a type constraint.
type AttributeBaseAst interface {
	Ast
	GetTypeName() ITypeName
}

// LabeledStatementAst is a loop or a switch.
type LabeledStatementAst interface {
	StatementAst
	GetLabel() string
}

type nodeBase struct{ Ext Extent }

func (n *nodeBase) Extent() Extent { return n.Ext }

type statementBase struct{ nodeBase }

func (*statementBase) isStatement() {}

type pipelineBase struct{ statementBase }

func (*pipelineBase) isPipelineBase() {}

type chainableBase struct{ pipelineBase }

func (*chainableBase) isChainable() {}

type expressionBase struct{ nodeBase }

func (*expressionBase) isCommandElement() {}
func (*expressionBase) isExpression()     {}

// ScriptBlockAst is a script, a script block, or a function body.
type ScriptBlockAst struct {
	nodeBase
	Attributes         []*AttributeAst
	UsingStatements    []*UsingStatementAst
	ParamBlock         *ParamBlockAst
	BeginBlock         *NamedBlockAst
	ProcessBlock       *NamedBlockAst
	EndBlock           *NamedBlockAst
	CleanBlock         *NamedBlockAst
	DynamicParamBlock  *NamedBlockAst
	ScriptRequirements *ScriptRequirements
}

// ScriptRequirements is what #requires lines ask for.
type ScriptRequirements struct {
	RequiredApplicationID string
	RequiredPSVersion     string
	RequiredPSEditions    []string
	// RequiredModules holds each module specification as written: a name, or a hashtable's text.
	RequiredModules     []string
	RequiredAssemblies  []string
	IsElevationRequired bool
}

type ParamBlockAst struct {
	nodeBase
	Attributes []*AttributeAst
	Parameters []*ParameterAst
}

type NamedBlockAst struct {
	nodeBase
	Unnamed    bool
	BlockKind  TokenKind
	Statements []StatementAst
	Traps      []*TrapStatementAst
}

type NamedAttributeArgumentAst struct {
	nodeBase
	ArgumentName      string
	Argument          ExpressionAst
	ExpressionOmitted bool
}

type AttributeAst struct {
	nodeBase
	TypeName            ITypeName
	PositionalArguments []ExpressionAst
	NamedArguments      []*NamedAttributeArgumentAst
}

func (a *AttributeAst) GetTypeName() ITypeName { return a.TypeName }

type TypeConstraintAst struct {
	nodeBase
	TypeName ITypeName
}

func (a *TypeConstraintAst) GetTypeName() ITypeName { return a.TypeName }

type ParameterAst struct {
	nodeBase
	Attributes   []AttributeBaseAst
	Name         *VariableExpressionAst
	DefaultValue ExpressionAst
}

type StatementBlockAst struct {
	nodeBase
	Statements []StatementAst
	Traps      []*TrapStatementAst
}

// TypeAttributes, PropertyAttributes and MethodAttributes are PowerShell's flag enums of the same names.
type TypeAttributes int

const (
	TypeAttributesClass     TypeAttributes = 0x01
	TypeAttributesInterface TypeAttributes = 0x02
	TypeAttributesEnum      TypeAttributes = 0x04
)

type PropertyAttributes int

const (
	PropertyAttributesPublic  PropertyAttributes = 0x01
	PropertyAttributesPrivate PropertyAttributes = 0x02
	PropertyAttributesStatic  PropertyAttributes = 0x10
	PropertyAttributesLiteral PropertyAttributes = 0x20
	PropertyAttributesHidden  PropertyAttributes = 0x40
)

type MethodAttributes int

const (
	MethodAttributesPublic  MethodAttributes = 0x01
	MethodAttributesPrivate MethodAttributes = 0x02
	MethodAttributesStatic  MethodAttributes = 0x10
	MethodAttributesHidden  MethodAttributes = 0x40
)

type TypeDefinitionAst struct {
	statementBase
	Name           string
	Attributes     []*AttributeAst
	BaseTypes      []*TypeConstraintAst
	Members        []MemberAst
	TypeAttributes TypeAttributes
}

// UsingStatementKind is PowerShell's UsingStatementKind.
type UsingStatementKind int

const (
	UsingAssembly UsingStatementKind = iota
	UsingCommand
	UsingModule
	UsingNamespace
	UsingType
)

type UsingStatementAst struct {
	statementBase
	UsingStatementKind  UsingStatementKind
	Name                *StringConstantExpressionAst
	Alias               *StringConstantExpressionAst
	ModuleSpecification *HashtableAst
}

type PropertyMemberAst struct {
	nodeBase
	Name               string
	PropertyType       *TypeConstraintAst
	Attributes         []*AttributeAst
	PropertyAttributes PropertyAttributes
	InitialValue       ExpressionAst
}

func (m *PropertyMemberAst) GetName() string { return m.Name }

type FunctionMemberAst struct {
	nodeBase
	Name             string
	Attributes       []*AttributeAst
	ReturnType       *TypeConstraintAst
	Parameters       []*ParameterAst
	Body             *ScriptBlockAst
	MethodAttributes MethodAttributes
}

func (m *FunctionMemberAst) GetName() string { return m.Name }

type FunctionDefinitionAst struct {
	statementBase
	IsFilter   bool
	IsWorkflow bool
	Name       string
	Parameters []*ParameterAst
	Body       *ScriptBlockAst
}

// IfClause is one condition and body of an if statement (Tuple<PipelineBaseAst, StatementBlockAst>).
type IfClause struct {
	Condition PipelineBaseAst
	Body      *StatementBlockAst
}

type IfStatementAst struct {
	statementBase
	Clauses    []IfClause
	ElseClause *StatementBlockAst
}

type DataStatementAst struct {
	statementBase
	Variable        string
	CommandsAllowed []ExpressionAst
	Body            *StatementBlockAst
}

type labeledBase struct {
	statementBase
	Label     string
	Condition PipelineBaseAst
}

func (l *labeledBase) GetLabel() string { return l.Label }

// ForEachFlags is PowerShell's ForEachFlags.
type ForEachFlags int

const ForEachParallel ForEachFlags = 0x01

type ForEachStatementAst struct {
	labeledBase
	Body          *StatementBlockAst
	Variable      *VariableExpressionAst
	ThrottleLimit ExpressionAst
	Flags         ForEachFlags
}

type ForStatementAst struct {
	labeledBase
	Body        *StatementBlockAst
	Initializer PipelineBaseAst
	Iterator    PipelineBaseAst
}

type DoWhileStatementAst struct {
	labeledBase
	Body *StatementBlockAst
}

type DoUntilStatementAst struct {
	labeledBase
	Body *StatementBlockAst
}

type WhileStatementAst struct {
	labeledBase
	Body *StatementBlockAst
}

// SwitchFlags is PowerShell's SwitchFlags.
type SwitchFlags int

const (
	SwitchFile          SwitchFlags = 0x01
	SwitchRegex         SwitchFlags = 0x02
	SwitchWildcard      SwitchFlags = 0x04
	SwitchExact         SwitchFlags = 0x08
	SwitchCaseSensitive SwitchFlags = 0x10
	SwitchParallel      SwitchFlags = 0x20
)

// SwitchClause is one case of a switch (Tuple<ExpressionAst, StatementBlockAst>).
type SwitchClause struct {
	Condition ExpressionAst
	Body      *StatementBlockAst
}

type SwitchStatementAst struct {
	labeledBase
	Flags   SwitchFlags
	Clauses []SwitchClause
	Default *StatementBlockAst
}

type CatchClauseAst struct {
	nodeBase
	CatchTypes []*TypeConstraintAst
	Body       *StatementBlockAst
}

// IsCatchAll reports a catch with no types.
func (c *CatchClauseAst) IsCatchAll() bool { return len(c.CatchTypes) == 0 }

type TryStatementAst struct {
	statementBase
	Body         *StatementBlockAst
	CatchClauses []*CatchClauseAst
	Finally      *StatementBlockAst
}

type TrapStatementAst struct {
	statementBase
	TrapType *TypeConstraintAst
	Body     *StatementBlockAst
}

type BreakStatementAst struct {
	statementBase
	Label ExpressionAst
}

type ContinueStatementAst struct {
	statementBase
	Label ExpressionAst
}

type ReturnStatementAst struct {
	statementBase
	Pipeline PipelineBaseAst
}

type ExitStatementAst struct {
	statementBase
	Pipeline PipelineBaseAst
}

type ThrowStatementAst struct {
	statementBase
	Pipeline PipelineBaseAst
}

// PipelineChainAst is a && or || chain.
type PipelineChainAst struct {
	chainableBase
	LhsPipelineChain ChainableAst
	RhsPipeline      *PipelineAst
	Operator         TokenKind
	Background       bool
}

type PipelineAst struct {
	chainableBase
	PipelineElements []CommandBaseAst
	Background       bool
}

type CommandParameterAst struct {
	nodeBase
	ParameterName string
	Argument      ExpressionAst
	ErrorPosition Extent
}

func (*CommandParameterAst) isCommandElement() {}

type CommandAst struct {
	statementBase
	CommandElements    []CommandElementAst
	InvocationOperator TokenKind
	Redirections       []RedirectionAst
}

func (c *CommandAst) GetRedirections() []RedirectionAst { return c.Redirections }

// GetCommandName is CommandAst.GetCommandName: the value of the first element when it is a string constant. ok is
// false where C# returns null, since an empty constant is still a name.
func (c *CommandAst) GetCommandName() (name string, ok bool) {
	if len(c.CommandElements) == 0 {
		return "", false
	}
	s, ok := c.CommandElements[0].(*StringConstantExpressionAst)
	if !ok {
		return "", false
	}
	return s.Value, true
}

type CommandExpressionAst struct {
	statementBase
	Expression   ExpressionAst
	Redirections []RedirectionAst
}

func (c *CommandExpressionAst) GetRedirections() []RedirectionAst { return c.Redirections }

type MergingRedirectionAst struct {
	nodeBase
	FromStream RedirectionStream
	ToStream   RedirectionStream
}

func (r *MergingRedirectionAst) GetFromStream() RedirectionStream { return r.FromStream }

type FileRedirectionAst struct {
	nodeBase
	FromStream RedirectionStream
	Location   ExpressionAst
	Append     bool
}

func (r *FileRedirectionAst) GetFromStream() RedirectionStream { return r.FromStream }

type AssignmentStatementAst struct {
	pipelineBase
	Left          ExpressionAst
	Operator      TokenKind
	Right         StatementAst
	ErrorPosition Extent
}

type BlockStatementAst struct {
	statementBase
	Body *StatementBlockAst
	// Kind is the keyword token (parallel or sequence).
	Kind *Token
}

type TernaryExpressionAst struct {
	expressionBase
	Condition ExpressionAst
	IfTrue    ExpressionAst
	IfFalse   ExpressionAst
}

type BinaryExpressionAst struct {
	expressionBase
	Operator      TokenKind
	Left          ExpressionAst
	Right         ExpressionAst
	ErrorPosition Extent
}

type UnaryExpressionAst struct {
	expressionBase
	TokenKind TokenKind
	Child     ExpressionAst
}

type AttributedExpressionAst struct {
	expressionBase
	Child     ExpressionAst
	Attribute AttributeBaseAst
}

type ConvertExpressionAst struct {
	expressionBase
	Child     ExpressionAst
	Attribute AttributeBaseAst
	Type      *TypeConstraintAst
}

type MemberExpressionAst struct {
	expressionBase
	Expression      ExpressionAst
	Member          CommandElementAst
	Static          bool
	NullConditional bool
}

type InvokeMemberExpressionAst struct {
	expressionBase
	Expression           ExpressionAst
	Member               CommandElementAst
	Static               bool
	NullConditional      bool
	GenericTypeArguments []ITypeName
	Arguments            []ExpressionAst
}

// BaseCtorInvokeMemberExpressionAst is a class constructor's call to its base constructor.
type BaseCtorInvokeMemberExpressionAst struct {
	InvokeMemberExpressionAst
}

type TypeExpressionAst struct {
	expressionBase
	TypeName ITypeName
}

type VariableExpressionAst struct {
	expressionBase
	VariablePath *VariablePath
	Splatted     bool
}

// ConstantExpressionAst is a number or other constant; Value holds the types Token.Value lists for a number.
type ConstantExpressionAst struct {
	expressionBase
	Value any
}

// StringConstantType is PowerShell's StringConstantType.
type StringConstantType int

const (
	SingleQuoted StringConstantType = iota
	SingleQuotedHereString
	DoubleQuoted
	DoubleQuotedHereString
	BareWord
)

type StringConstantExpressionAst struct {
	expressionBase
	StringConstantType StringConstantType
	Value              string
}

type ExpandableStringExpressionAst struct {
	expressionBase
	Value              string
	StringConstantType StringConstantType
	NestedExpressions  []ExpressionAst
	// FormatString is the string with each nested expression replaced by {n}, as PowerShell keeps it internally.
	FormatString string
}

type ScriptBlockExpressionAst struct {
	expressionBase
	ScriptBlock *ScriptBlockAst
}

type ArrayLiteralAst struct {
	expressionBase
	Elements []ExpressionAst
}

// KeyValuePair is one hashtable entry (Tuple<ExpressionAst, StatementAst>).
type KeyValuePair struct {
	Key   ExpressionAst
	Value StatementAst
}

type HashtableAst struct {
	expressionBase
	KeyValuePairs []KeyValuePair
}

type ArrayExpressionAst struct {
	expressionBase
	SubExpression *StatementBlockAst
}

type ParenExpressionAst struct {
	expressionBase
	Pipeline PipelineBaseAst
}

type SubExpressionAst struct {
	expressionBase
	SubExpression *StatementBlockAst
}

type UsingExpressionAst struct {
	expressionBase
	SubExpression ExpressionAst
}

type IndexExpressionAst struct {
	expressionBase
	Target          ExpressionAst
	Index           ExpressionAst
	NullConditional bool
}

var (
	_ StatementAst        = (*TypeDefinitionAst)(nil)
	_ StatementAst        = (*UsingStatementAst)(nil)
	_ StatementAst        = (*FunctionDefinitionAst)(nil)
	_ StatementAst        = (*IfStatementAst)(nil)
	_ StatementAst        = (*DataStatementAst)(nil)
	_ LabeledStatementAst = (*ForEachStatementAst)(nil)
	_ LabeledStatementAst = (*ForStatementAst)(nil)
	_ LabeledStatementAst = (*DoWhileStatementAst)(nil)
	_ LabeledStatementAst = (*DoUntilStatementAst)(nil)
	_ LabeledStatementAst = (*WhileStatementAst)(nil)
	_ LabeledStatementAst = (*SwitchStatementAst)(nil)
	_ StatementAst        = (*TryStatementAst)(nil)
	_ StatementAst        = (*TrapStatementAst)(nil)
	_ StatementAst        = (*BreakStatementAst)(nil)
	_ StatementAst        = (*ContinueStatementAst)(nil)
	_ StatementAst        = (*ReturnStatementAst)(nil)
	_ StatementAst        = (*ExitStatementAst)(nil)
	_ StatementAst        = (*ThrowStatementAst)(nil)
	_ StatementAst        = (*BlockStatementAst)(nil)
	_ ChainableAst        = (*PipelineChainAst)(nil)
	_ ChainableAst        = (*PipelineAst)(nil)
	_ PipelineBaseAst     = (*AssignmentStatementAst)(nil)
	_ CommandBaseAst      = (*CommandAst)(nil)
	_ CommandBaseAst      = (*CommandExpressionAst)(nil)
	_ CommandElementAst   = (*CommandParameterAst)(nil)
	_ RedirectionAst      = (*MergingRedirectionAst)(nil)
	_ RedirectionAst      = (*FileRedirectionAst)(nil)
	_ MemberAst           = (*PropertyMemberAst)(nil)
	_ MemberAst           = (*FunctionMemberAst)(nil)
	_ AttributeBaseAst    = (*AttributeAst)(nil)
	_ AttributeBaseAst    = (*TypeConstraintAst)(nil)
	_ ExpressionAst       = (*TernaryExpressionAst)(nil)
	_ ExpressionAst       = (*BinaryExpressionAst)(nil)
	_ ExpressionAst       = (*UnaryExpressionAst)(nil)
	_ ExpressionAst       = (*AttributedExpressionAst)(nil)
	_ ExpressionAst       = (*ConvertExpressionAst)(nil)
	_ ExpressionAst       = (*MemberExpressionAst)(nil)
	_ ExpressionAst       = (*InvokeMemberExpressionAst)(nil)
	_ ExpressionAst       = (*BaseCtorInvokeMemberExpressionAst)(nil)
	_ ExpressionAst       = (*TypeExpressionAst)(nil)
	_ ExpressionAst       = (*VariableExpressionAst)(nil)
	_ ExpressionAst       = (*ConstantExpressionAst)(nil)
	_ ExpressionAst       = (*StringConstantExpressionAst)(nil)
	_ ExpressionAst       = (*ExpandableStringExpressionAst)(nil)
	_ ExpressionAst       = (*ScriptBlockExpressionAst)(nil)
	_ ExpressionAst       = (*ArrayLiteralAst)(nil)
	_ ExpressionAst       = (*HashtableAst)(nil)
	_ ExpressionAst       = (*ArrayExpressionAst)(nil)
	_ ExpressionAst       = (*ParenExpressionAst)(nil)
	_ ExpressionAst       = (*SubExpressionAst)(nil)
	_ ExpressionAst       = (*UsingExpressionAst)(nil)
	_ ExpressionAst       = (*IndexExpressionAst)(nil)
)
