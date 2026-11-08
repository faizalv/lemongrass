// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// Constructors for the nodes the statement half creates, porting the logic of their ast.cs constructors: where C#
// substitutes an empty collection for null the slice is empty and non-nil, and where C# leaves it null it is nil,
// so the tree reads as PowerShell's does. The expression half writes the constructors for its own nodes.

// emptyOr returns s, or an empty non-nil slice for nil, as C# substitutes an empty ReadOnlyCollection for null.
func emptyOr[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// nilIfEmpty returns nil for an empty slice, as C# leaves a collection null when it is given none.
func nilIfEmpty[T any](s []T) []T {
	if len(s) == 0 {
		return nil
	}
	return s
}

// blankToEmpty is string.IsNullOrWhiteSpace mapping to null: a label or data variable of only white space is no
// name at all. Go has no null string, so the empty string stands for it.
func blankToEmpty(s string) string {
	for _, r := range s {
		if !isDotNetWhiteSpace(r) {
			return s
		}
	}
	return ""
}

// isDotNetWhiteSpace is char.IsWhiteSpace: the Unicode space separators, line and paragraph separators, and the
// controls U+0009 to U+000D, U+0085.
func isDotNetWhiteSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// newScriptBlockAstNamed is ScriptBlockAst(extent, usingStatements, attributes, paramBlock, beginBlock,
// processBlock, endBlock, cleanBlock, dynamicParamBlock).
func newScriptBlockAstNamed(ext Extent, usingStatements []*UsingStatementAst, attributes []*AttributeAst, paramBlock *ParamBlockAst,
	beginBlock, processBlock, endBlock, cleanBlock, dynamicParamBlock *NamedBlockAst) *ScriptBlockAst {
	n := &ScriptBlockAst{
		UsingStatements:   emptyOr(usingStatements),
		Attributes:        emptyOr(attributes),
		ParamBlock:        paramBlock,
		BeginBlock:        beginBlock,
		ProcessBlock:      processBlock,
		EndBlock:          endBlock,
		CleanBlock:        cleanBlock,
		DynamicParamBlock: dynamicParamBlock,
	}
	n.Ext = ext
	return n
}

// newScriptBlockAst is ScriptBlockAst(extent, usingStatements, attributes, paramBlock, statements, isFilter,
// isConfiguration): the statements become an unnamed process block for a filter, else an unnamed end block, with
// the statement block's extent.
func newScriptBlockAst(ext Extent, usingStatements []*UsingStatementAst, attributes []*AttributeAst, paramBlock *ParamBlockAst,
	statements *StatementBlockAst, isFilter bool) *ScriptBlockAst {
	n := &ScriptBlockAst{
		UsingStatements: emptyOr(usingStatements),
		Attributes:      emptyOr(attributes),
		ParamBlock:      paramBlock,
	}
	n.Ext = ext
	if isFilter {
		n.ProcessBlock = newNamedBlockAst(statements.Ext, Process, statements, true)
	} else {
		n.EndBlock = newNamedBlockAst(statements.Ext, End, statements, true)
	}
	return n
}

// newNamedBlockAst is NamedBlockAst(extent, blockName, statementBlock, unnamed): it takes the block's statements and
// traps, Traps staying nil when there are none.
func newNamedBlockAst(ext Extent, blockName TokenKind, statementBlock *StatementBlockAst, unnamed bool) *NamedBlockAst {
	n := &NamedBlockAst{
		Unnamed:    unnamed,
		BlockKind:  blockName,
		Statements: statementBlock.Statements,
		Traps:      nilIfEmpty(statementBlock.Traps),
	}
	n.Ext = ext
	return n
}

// newStatementBlockAst is StatementBlockAst(extent, statements, traps).
func newStatementBlockAst(ext Extent, statements []StatementAst, traps []*TrapStatementAst) *StatementBlockAst {
	n := &StatementBlockAst{Statements: emptyOr(statements), Traps: nilIfEmpty(traps)}
	n.Ext = ext
	return n
}

// newCommandExpressionAst is CommandExpressionAst(extent, expression, redirections).
func newCommandExpressionAst(ext Extent, expr ExpressionAst, redirections []RedirectionAst) *CommandExpressionAst {
	n := &CommandExpressionAst{Expression: expr, Redirections: emptyOr(redirections)}
	n.Ext = ext
	return n
}

// newCommandAst is CommandAst(extent, commandElements, invocationOperator, redirections).
func newCommandAst(ext Extent, elements []CommandElementAst, invocationOperator TokenKind, redirections []RedirectionAst) *CommandAst {
	n := &CommandAst{CommandElements: elements, InvocationOperator: invocationOperator, Redirections: emptyOr(redirections)}
	n.Ext = ext
	return n
}

// newCommandParameterAst is CommandParameterAst(extent, parameterName, argument, errorPosition).
func newCommandParameterAst(ext Extent, parameterName string, argument ExpressionAst, errorPosition Extent) *CommandParameterAst {
	n := &CommandParameterAst{ParameterName: parameterName, Argument: argument, ErrorPosition: errorPosition}
	n.Ext = ext
	return n
}

// newFileRedirectionAst is FileRedirectionAst(extent, stream, file, append).
func newFileRedirectionAst(ext Extent, stream RedirectionStream, file ExpressionAst, appendTo bool) *FileRedirectionAst {
	n := &FileRedirectionAst{FromStream: stream, Location: file, Append: appendTo}
	n.Ext = ext
	return n
}

// newMergingRedirectionAst is MergingRedirectionAst(extent, from, to).
func newMergingRedirectionAst(ext Extent, from, to RedirectionStream) *MergingRedirectionAst {
	n := &MergingRedirectionAst{FromStream: from, ToStream: to}
	n.Ext = ext
	return n
}

// newPipelineAst is PipelineAst(extent, pipelineElements, background).
func newPipelineAst(ext Extent, elements []CommandBaseAst, background bool) *PipelineAst {
	n := &PipelineAst{PipelineElements: elements, Background: background}
	n.Ext = ext
	return n
}

// newPipelineChainAst is PipelineChainAst(extent, lhsChain, rhsPipeline, chainOperator, background).
func newPipelineChainAst(ext Extent, lhs ChainableAst, rhs *PipelineAst, op TokenKind, background bool) *PipelineChainAst {
	n := &PipelineChainAst{LhsPipelineChain: lhs, RhsPipeline: rhs, Operator: op, Background: background}
	n.Ext = ext
	return n
}

// newAssignmentStatementAst is AssignmentStatementAst(extent, left, operator, right, errorPosition): a right side
// that is a lone expression in a foreground pipeline is kept as the CommandExpressionAst itself.
func newAssignmentStatementAst(ext Extent, left ExpressionAst, op TokenKind, right StatementAst, errorPosition Extent) *AssignmentStatementAst {
	if pl, ok := right.(*PipelineAst); ok && !pl.Background && len(pl.PipelineElements) == 1 {
		if ce, ok := pl.PipelineElements[0].(*CommandExpressionAst); ok {
			right = ce
		}
	}
	n := &AssignmentStatementAst{Left: left, Operator: op, Right: right, ErrorPosition: errorPosition}
	n.Ext = ext
	return n
}

// newFunctionDefinitionAst is FunctionDefinitionAst(extent, isFilter, isWorkflow, functionNameToken, parameters,
// body): a Generic name token gives its value, any other its text; Parameters stays nil when there are none.
func newFunctionDefinitionAst(ext Extent, isFilter, isWorkflow bool, nameToken *Token, params []*ParameterAst, body *ScriptBlockAst) *FunctionDefinitionAst {
	name := nameToken.Text
	if nameToken.Kind == Generic {
		name = nameToken.StringValue
	}
	n := &FunctionDefinitionAst{IsFilter: isFilter, IsWorkflow: isWorkflow, Name: name, Parameters: nilIfEmpty(params), Body: body}
	n.Ext = ext
	return n
}

// newIfStatementAst is IfStatementAst(extent, clauses, elseClause).
func newIfStatementAst(ext Extent, clauses []IfClause, elseClause *StatementBlockAst) *IfStatementAst {
	n := &IfStatementAst{Clauses: clauses, ElseClause: elseClause}
	n.Ext = ext
	return n
}

// newDataStatementAst is DataStatementAst(extent, variableName, commandsAllowed, body).
func newDataStatementAst(ext Extent, variableName string, commandsAllowed []ExpressionAst, body *StatementBlockAst) *DataStatementAst {
	n := &DataStatementAst{Variable: blankToEmpty(variableName), CommandsAllowed: emptyOr(commandsAllowed), Body: body}
	n.Ext = ext
	return n
}

// labeled is LabeledStatementAst(extent, label, condition).
func labeled(ext Extent, label string, condition PipelineBaseAst) labeledBase {
	l := labeledBase{Label: blankToEmpty(label), Condition: condition}
	l.Ext = ext
	return l
}

// newForEachStatementAst is ForEachStatementAst(extent, label, flags, throttleLimit, variable, expression, body).
func newForEachStatementAst(ext Extent, label string, flags ForEachFlags, throttleLimit ExpressionAst, variable *VariableExpressionAst,
	expression PipelineBaseAst, body *StatementBlockAst) *ForEachStatementAst {
	return &ForEachStatementAst{labeledBase: labeled(ext, label, expression), Body: body, Variable: variable, ThrottleLimit: throttleLimit, Flags: flags}
}

// newForStatementAst is ForStatementAst(extent, label, initializer, condition, iterator, body).
func newForStatementAst(ext Extent, label string, initializer, condition, iterator PipelineBaseAst, body *StatementBlockAst) *ForStatementAst {
	return &ForStatementAst{labeledBase: labeled(ext, label, condition), Body: body, Initializer: initializer, Iterator: iterator}
}

// newWhileStatementAst is WhileStatementAst(extent, label, condition, body).
func newWhileStatementAst(ext Extent, label string, condition PipelineBaseAst, body *StatementBlockAst) *WhileStatementAst {
	return &WhileStatementAst{labeledBase: labeled(ext, label, condition), Body: body}
}

// newDoWhileStatementAst is DoWhileStatementAst(extent, label, condition, body).
func newDoWhileStatementAst(ext Extent, label string, condition PipelineBaseAst, body *StatementBlockAst) *DoWhileStatementAst {
	return &DoWhileStatementAst{labeledBase: labeled(ext, label, condition), Body: body}
}

// newDoUntilStatementAst is DoUntilStatementAst(extent, label, condition, body).
func newDoUntilStatementAst(ext Extent, label string, condition PipelineBaseAst, body *StatementBlockAst) *DoUntilStatementAst {
	return &DoUntilStatementAst{labeledBase: labeled(ext, label, condition), Body: body}
}

// newSwitchStatementAst is SwitchStatementAst(extent, label, condition, flags, clauses, default).
func newSwitchStatementAst(ext Extent, label string, condition PipelineBaseAst, flags SwitchFlags, clauses []SwitchClause, def *StatementBlockAst) *SwitchStatementAst {
	return &SwitchStatementAst{labeledBase: labeled(ext, label, condition), Flags: flags, Clauses: emptyOr(clauses), Default: def}
}

// newCatchClauseAst is CatchClauseAst(extent, catchTypes, body).
func newCatchClauseAst(ext Extent, catchTypes []*TypeConstraintAst, body *StatementBlockAst) *CatchClauseAst {
	n := &CatchClauseAst{CatchTypes: emptyOr(catchTypes), Body: body}
	n.Ext = ext
	return n
}

// newTryStatementAst is TryStatementAst(extent, body, catchClauses, finally).
func newTryStatementAst(ext Extent, body *StatementBlockAst, catchClauses []*CatchClauseAst, finally *StatementBlockAst) *TryStatementAst {
	n := &TryStatementAst{Body: body, CatchClauses: emptyOr(catchClauses), Finally: finally}
	n.Ext = ext
	return n
}

// newTrapStatementAst is TrapStatementAst(extent, trapType, body).
func newTrapStatementAst(ext Extent, trapType *TypeConstraintAst, body *StatementBlockAst) *TrapStatementAst {
	n := &TrapStatementAst{TrapType: trapType, Body: body}
	n.Ext = ext
	return n
}

// newBreakStatementAst is BreakStatementAst(extent, label).
func newBreakStatementAst(ext Extent, label ExpressionAst) *BreakStatementAst {
	n := &BreakStatementAst{Label: label}
	n.Ext = ext
	return n
}

// newContinueStatementAst is ContinueStatementAst(extent, label).
func newContinueStatementAst(ext Extent, label ExpressionAst) *ContinueStatementAst {
	n := &ContinueStatementAst{Label: label}
	n.Ext = ext
	return n
}

// newReturnStatementAst is ReturnStatementAst(extent, pipeline).
func newReturnStatementAst(ext Extent, pipeline PipelineBaseAst) *ReturnStatementAst {
	n := &ReturnStatementAst{Pipeline: pipeline}
	n.Ext = ext
	return n
}

// newExitStatementAst is ExitStatementAst(extent, pipeline).
func newExitStatementAst(ext Extent, pipeline PipelineBaseAst) *ExitStatementAst {
	n := &ExitStatementAst{Pipeline: pipeline}
	n.Ext = ext
	return n
}

// newThrowStatementAst is ThrowStatementAst(extent, pipeline).
func newThrowStatementAst(ext Extent, pipeline PipelineBaseAst) *ThrowStatementAst {
	n := &ThrowStatementAst{Pipeline: pipeline}
	n.Ext = ext
	return n
}

// newBlockStatementAst is BlockStatementAst(extent, kind, body).
func newBlockStatementAst(ext Extent, kind *Token, body *StatementBlockAst) *BlockStatementAst {
	n := &BlockStatementAst{Kind: kind, Body: body}
	n.Ext = ext
	return n
}
