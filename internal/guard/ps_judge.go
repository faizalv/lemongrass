// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"errors"
	"fmt"

	"github.com/faizalv/lemongrass/internal/psparse"
)

// The PowerShell judge turns psparse's tree into the records the bash parser produces, so scriptVerdicts judges a
// PowerShell command against the same catalog. The gate cannot know which PowerShell runs a call, so a command is
// recorded once for each meaning its name has under 5.1, pwsh on Windows and pwsh on Unix, and once for each way
// those editions read its strings; any deny among them wins. Every branch is walked, as the bash side walks every
// branch, and what the judge cannot reason about becomes a refusal in shellScript.Refusals.

// psEditionList is every edition in a fixed order, for per-edition pipeline name lists.
var psEditionList = []psEdition{psWin51, psWin7, psUnix7}

type psJudge struct {
	s     *shellScript
	src   func(psparse.Extent) string
	depth int
	err   error
	// seen dedupes the records one command yields across its meanings and readings; nil outside a command.
	seen map[string]bool
}

// addPowerShell parses text as PowerShell and appends every record and refusal it yields; a parse error is
// returned, as add returns one, since PowerShell runs nothing from input it cannot parse.
func (s *shellScript) addPowerShell(text string, depth int) error {
	if depth > maxShellDepth {
		return errors.New("guard: PowerShell command nests too deeply to inspect")
	}
	res, err := psparse.Parse(text, psparse.DefaultLimits)
	if err != nil {
		return err
	}
	return s.judgePowerShell(res.Ast, res.Text, depth)
}

// judgePowerShell walks a parsed script; src returns the source text of an extent.
func (s *shellScript) judgePowerShell(ast *psparse.ScriptBlockAst, src func(psparse.Extent) string, depth int) error {
	j := &psJudge{s: s, src: src, depth: depth}
	j.scriptBlock(ast)
	return j.err
}

func (j *psJudge) text(node psparse.Ast) string {
	if j.src == nil {
		return ""
	}
	return j.src(node.Extent())
}

func (j *psJudge) fail(err error) {
	if err != nil && j.err == nil {
		j.err = err
	}
}

func (j *psJudge) scriptBlock(sb *psparse.ScriptBlockAst) {
	if sb == nil {
		return
	}
	if req := sb.ScriptRequirements; req != nil && (len(req.RequiredModules) > 0 || len(req.RequiredAssemblies) > 0) {
		j.refuse("powershell-construct", "#requires loading a module or assembly", psGuideModule)
	}
	for _, using := range sb.UsingStatements {
		j.statement(using)
	}
	for _, attribute := range sb.Attributes {
		j.attribute(attribute)
	}
	if sb.ParamBlock != nil {
		for _, attribute := range sb.ParamBlock.Attributes {
			j.attribute(attribute)
		}
		j.parameters(sb.ParamBlock.Parameters)
	}
	for _, block := range []*psparse.NamedBlockAst{sb.DynamicParamBlock, sb.BeginBlock, sb.ProcessBlock, sb.EndBlock, sb.CleanBlock} {
		if block != nil {
			j.statements(block.Statements, block.Traps)
		}
	}
	if sb.CleanBlock != nil {
		// pwsh 7.3+ runs clean { } as a named block; 5.1 reads it as a command named clean with a script block argument.
		j.fail(j.s.addCall("clean", false, []string{psDynamicText}, []bool{true}, false, j.depth))
	}
}

func (j *psJudge) parameters(params []*psparse.ParameterAst) {
	for _, param := range params {
		for _, attribute := range param.Attributes {
			j.attribute(attribute)
		}
		j.walk(param.DefaultValue)
	}
}

func (j *psJudge) attribute(attribute psparse.AttributeBaseAst) {
	a, ok := attribute.(*psparse.AttributeAst)
	if !ok || a == nil {
		return
	}
	for _, arg := range a.PositionalArguments {
		j.walk(arg)
	}
	for _, named := range a.NamedArguments {
		j.walk(named.Argument)
	}
}

func (j *psJudge) statementBlock(block *psparse.StatementBlockAst) {
	if block != nil {
		j.statements(block.Statements, block.Traps)
	}
}

func (j *psJudge) statements(statements []psparse.StatementAst, traps []*psparse.TrapStatementAst) {
	for _, trap := range traps {
		j.statement(trap)
	}
	for _, statement := range statements {
		j.statement(statement)
	}
}

// statement walks one statement and every branch in it, whether or not that branch would run.
func (j *psJudge) statement(statement psparse.StatementAst) {
	switch s := statement.(type) {
	case nil:
	case *psparse.PipelineAst:
		j.pipeline(s)
	case *psparse.PipelineChainAst:
		j.statement(s.LhsPipelineChain)
		if s.RhsPipeline != nil {
			j.pipeline(s.RhsPipeline)
		}
		if s.Background {
			j.refuse("powershell-construct", "a trailing &", psGuideJob)
		}
	case *psparse.AssignmentStatementAst:
		j.assignment(s)
	case *psparse.CommandExpressionAst:
		j.walk(s.Expression)
		j.redirections(s.Redirections)
	case *psparse.CommandAst:
		j.command(s, 0, nil)
	case *psparse.IfStatementAst:
		for _, clause := range s.Clauses {
			j.statement(clause.Condition)
			j.statementBlock(clause.Body)
		}
		j.statementBlock(s.ElseClause)
	case *psparse.SwitchStatementAst:
		j.statement(s.Condition)
		if s.Flags&psparse.SwitchFile != 0 {
			j.switchFile(s.Condition)
		}
		for _, clause := range s.Clauses {
			j.walk(clause.Condition)
			j.statementBlock(clause.Body)
		}
		j.statementBlock(s.Default)
	case *psparse.ForEachStatementAst:
		j.statement(s.Condition)
		j.walk(s.ThrottleLimit)
		j.statementBlock(s.Body)
	case *psparse.ForStatementAst:
		j.statement(s.Initializer)
		j.statement(s.Condition)
		j.statement(s.Iterator)
		j.statementBlock(s.Body)
	case *psparse.WhileStatementAst:
		j.statement(s.Condition)
		j.statementBlock(s.Body)
	case *psparse.DoWhileStatementAst:
		j.statement(s.Condition)
		j.statementBlock(s.Body)
	case *psparse.DoUntilStatementAst:
		j.statement(s.Condition)
		j.statementBlock(s.Body)
	case *psparse.TryStatementAst:
		j.statementBlock(s.Body)
		for _, catch := range s.CatchClauses {
			j.statementBlock(catch.Body)
		}
		j.statementBlock(s.Finally)
	case *psparse.TrapStatementAst:
		j.statementBlock(s.Body)
	case *psparse.ThrowStatementAst:
		j.statement(s.Pipeline)
	case *psparse.ReturnStatementAst:
		j.statement(s.Pipeline)
	case *psparse.ExitStatementAst:
		j.statement(s.Pipeline)
	case *psparse.BreakStatementAst:
		j.walk(s.Label)
	case *psparse.ContinueStatementAst:
		j.walk(s.Label)
	case *psparse.DataStatementAst:
		for _, allowed := range s.CommandsAllowed {
			j.walk(allowed)
		}
		j.statementBlock(s.Body)
	case *psparse.BlockStatementAst:
		j.statementBlock(s.Body)
	case *psparse.FunctionDefinitionAst:
		kind := "function"
		switch {
		case s.IsFilter:
			kind = "filter"
		case s.IsWorkflow:
			// pwsh 7 rejects a workflow only in its post-parse checks, which are not ported, and 5.1 runs it.
			kind = "workflow"
		}
		j.refuse("powershell-construct", "a "+kind+" definition "+s.Name, psGuideDefinition)
	case *psparse.TypeDefinitionAst:
		kind := "class"
		if s.TypeAttributes&psparse.TypeAttributesEnum != 0 {
			kind = "enum"
		}
		j.refuse("powershell-construct", "a "+kind+" definition "+s.Name, psGuideDefinition)
	case *psparse.UsingStatementAst:
		if s.UsingStatementKind == psparse.UsingModule || s.UsingStatementKind == psparse.UsingAssembly {
			j.refuse("powershell-construct", j.text(s), psGuideModule)
		}
	default:
		j.refuse("powershell-construct", fmt.Sprintf("a statement of type %T", statement), psGuideUnknown)
	}
}

// switchFile records the read a switch -File makes of the file its condition names.
func (j *psJudge) switchFile(condition psparse.PipelineBaseAst) {
	pipeline, ok := condition.(*psparse.PipelineAst)
	if !ok || len(pipeline.PipelineElements) != 1 {
		return
	}
	expr, ok := pipeline.PipelineElements[0].(*psparse.CommandExpressionAst)
	if !ok {
		return
	}
	values := j.values(expr.Expression, psModern)
	j.cmdletEffect("get-content", "switch -File", []psElement{{Param: "Path", Values: values}}, false)
}

// assignment walks both sides. A variable on a provider drive is that item: ${C:\x} = 'y' writes the file as
// Set-Content would, and $function:f or $alias:x redefine a name.
func (j *psJudge) assignment(a *psparse.AssignmentStatementAst) {
	if v := psAssignedVariable(a.Left); v != nil && v.VariablePath != nil && v.VariablePath.IsDriveQualified() {
		path := v.VariablePath
		drive := path.DriveName()
		switch {
		case psparse.OrdinalEqualFold(drive, "function") || psparse.OrdinalEqualFold(drive, "alias"):
			j.refuse("name-redefinition", "an assignment to $"+path.UserPath, psGuideAlias)
		case !psPlainVariableDrive(drive):
			if a.Operator != psparse.Equals {
				j.cmdletEffect("get-content", "${"+path.UserPath+"}", psPathElements(path.UserPath), false)
			}
			j.cmdletEffect("set-content", "${"+path.UserPath+"}", psPathElements(path.UserPath), false)
		}
	} else {
		j.walk(a.Left)
	}
	j.statement(a.Right)
}

// psAssignedVariable is the variable an assignment target names under any casts and attributes, or nil.
func psAssignedVariable(left psparse.ExpressionAst) *psparse.VariableExpressionAst {
	for {
		switch e := left.(type) {
		case *psparse.VariableExpressionAst:
			return e
		case *psparse.ConvertExpressionAst:
			left = e.Child
		case *psparse.AttributedExpressionAst:
			left = e.Child
		default:
			return nil
		}
	}
}

func (j *psJudge) pipeline(p *psparse.PipelineAst) {
	if p.Background {
		j.refuse("powershell-construct", "a trailing &", psGuideJob)
	}
	names := make([][]string, len(psEditionList))
	for i, element := range p.PipelineElements {
		var stage map[psEdition]string
		switch e := element.(type) {
		case *psparse.CommandAst:
			var prev psparse.CommandBaseAst
			if i > 0 {
				prev = p.PipelineElements[i-1]
			}
			stage = j.command(e, i, prev)
		case *psparse.CommandExpressionAst:
			j.walk(e.Expression)
			j.redirections(e.Redirections)
		default:
			j.refuse("powershell-construct", fmt.Sprintf("a pipeline element of type %T", element), psGuideUnknown)
		}
		for k, edition := range psEditionList {
			names[k] = append(names[k], stage[edition])
		}
	}
	if len(p.PipelineElements) < 2 {
		return
	}
	recorded := map[string]bool{}
	for _, list := range names {
		key := fmt.Sprintf("%q", list)
		if !recorded[key] {
			recorded[key] = true
			j.s.Pipelines = append(j.s.Pipelines, list)
		}
	}
}

func (j *psJudge) redirections(redirections []psparse.RedirectionAst) {
	for _, redirection := range redirections {
		file, ok := redirection.(*psparse.FileRedirectionAst)
		if !ok {
			continue
		}
		j.walk(file.Location)
		if psIsNull(file.Location) {
			continue
		}
		// pwsh 7.6 reads > variable:x as a variable; 5.1 writes a file of that name, and that reading is the one judged.
		for _, value := range j.values(file.Location, psModern) {
			j.record(shellRedirect{Path: value.Text, Write: true})
		}
	}
}

// command records one command under every meaning and reading, and returns its name per edition for the pipeline
// lists. stage is its position in its pipeline; prev is the stage before it, or nil.
func (j *psJudge) command(cmd *psparse.CommandAst, stage int, prev psparse.CommandBaseAst) map[psEdition]string {
	for _, element := range cmd.CommandElements {
		switch e := element.(type) {
		case *psparse.CommandParameterAst:
			j.walk(e.Argument)
		case psparse.ExpressionAst:
			j.walk(e)
		}
	}
	j.redirections(cmd.Redirections)
	if len(cmd.CommandElements) == 0 {
		return nil
	}
	first := cmd.CommandElements[0]
	if _, ok := first.(*psparse.ScriptBlockExpressionAst); ok && (cmd.InvocationOperator == psparse.Ampersand || cmd.InvocationOperator == psparse.Dot) {
		return nil
	}
	nameAst, ok := first.(*psparse.StringConstantExpressionAst)
	if !ok || j.legacyString(nameAst, nameAst.Value, nil) != nameAst.Value {
		j.refuse("dynamic-command", "the command name "+j.text(first), psGuideDynamic)
		return nil
	}
	name := nameAst.Value
	modern, legacy, differs := j.elements(cmd.CommandElements[1:])
	for _, element := range cmd.CommandElements[1:] {
		if s, ok := element.(*psparse.StringConstantExpressionAst); ok && psparse.OrdinalEqualFold(s.Value, "--%") {
			j.refuse("powershell-construct", "--% after "+name, psGuideStopParse)
			return nil
		}
	}

	names := map[psEdition]string{}
	j.seen = map[string]bool{}
	defer func() { j.seen = nil }()
	for _, meaning := range psResolveName(name) {
		readings := [][]psElement{modern}
		if differs {
			switch {
			case meaning.Editions == psWin51:
				readings = [][]psElement{legacy}
			case meaning.Editions&psWin51 != 0:
				readings = [][]psElement{modern, legacy}
			}
		}
		stageName := meaning.Cmdlet
		if meaning.Cmdlet == "" {
			stageName = commandBase(meaning.Native)
		}
		for _, edition := range psEditionList {
			if meaning.Editions&edition != 0 {
				names[edition] = stageName
			}
		}
		for _, elems := range readings {
			if meaning.Cmdlet != "" {
				all := append(append([]psElement(nil), meaning.Fixed...), elems...)
				j.cmdletEffect(meaning.Cmdlet, name, all, stage > 0)
				continue
			}
			j.native(meaning.Native, elems, prev)
		}
	}
	return names
}

// elements reads the arguments under both readings; differs is false when they read alike.
func (j *psJudge) elements(elements []psparse.CommandElementAst) (modern, legacy []psElement, differs bool) {
	for _, element := range elements {
		var m, l psElement
		switch e := element.(type) {
		case *psparse.CommandParameterAst:
			m = psElement{Param: e.ParameterName, Colon: e.Argument != nil, Values: j.values(e.Argument, psModern)}
			l = psElement{Param: e.ParameterName, Colon: e.Argument != nil, Values: j.values(e.Argument, psLegacy)}
		case *psparse.VariableExpressionAst:
			if e.Splatted {
				m = psElement{Splat: true, Values: []psValue{{Text: "@" + psVariableName(e), Dynamic: true}}}
				l = m
				break
			}
			m = psElement{Values: j.values(e, psModern)}
			l = m
		case psparse.ExpressionAst:
			m = psElement{Values: j.values(e, psModern)}
			l = psElement{Values: j.values(e, psLegacy)}
		default:
			j.refuse("powershell-construct", fmt.Sprintf("a command element of type %T", element), psGuideUnknown)
			continue
		}
		if fmt.Sprint(m) != fmt.Sprint(l) {
			differs = true
		}
		modern = append(modern, m)
		legacy = append(legacy, l)
	}
	return modern, legacy, differs
}

func psVariableName(v *psparse.VariableExpressionAst) string {
	if v.VariablePath == nil {
		return ""
	}
	return v.VariablePath.UserPath
}

// cmdletEffect records a cmdlet call: the call itself under its canonical name, so the catalog's cmdlet forms match,
// its POSIX equivalents, and the refusals for its name and its binding. pipelineInput marks a later pipeline stage,
// which binds its paths from the pipeline when none is written.
func (j *psJudge) cmdletEffect(cmdlet, written string, elems []psElement, pipelineInput bool) {
	if refusal, ok := psRefusedCmdlets[cmdlet]; ok {
		j.refuse(refusal.rule, written, refusal.guide)
	}
	args, dynamic := psCmdletArgs(cmdlet, elems)
	j.record(shellCall{Name: cmdlet, Path: written, Args: args, ArgDynamic: dynamic})
	bound := psBind(cmdlet, elems)
	if bound.Refusal != "" {
		j.refuse("powershell-construct", written, bound.Refusal)
	}
	calls, redirects := psEquivalents(bound, pipelineInput)
	for _, call := range calls {
		j.record(call)
	}
	for _, redirect := range redirects {
		j.record(redirect)
	}
	if cmdlet == "switch-process" {
		j.switchProcess(elems)
	}
}

// switchProcess unwraps Switch-Process (exec on Unix), which replaces PowerShell with the command its remaining
// arguments name, as addCall unwraps bash's exec: the first value is the program and every element after it, a
// parameter-like word included, is that program's argument.
func (j *psJudge) switchProcess(elems []psElement) {
	for i, element := range elems {
		if element.Splat {
			j.refuse("powershell-construct", "a splat into exec", psGuideUnknown)
			return
		}
		if element.Param != "" && !element.Colon {
			continue
		}
		if len(element.Values) == 0 {
			continue
		}
		program := element.Values[0]
		if program.Dynamic {
			j.refuse("dynamic-command", "the command exec runs", psGuideDynamic)
			return
		}
		rest := append([]psElement(nil), elems[i+1:]...)
		if len(element.Values) > 1 {
			rest = append([]psElement{{Values: element.Values[1:]}}, rest...)
		}
		j.native(program.Text, rest, nil)
		return
	}
}

// native records a native program call for every argv the passing models give it, through addCall so wrappers,
// bash -c and interpreters unwrap as they do for bash, and code piped into an interpreter becomes a program.
func (j *psJudge) native(name string, elems []psElement, prev psparse.CommandBaseAst) {
	readings, refusal := psNativeArgv(name, elems)
	if refusal != "" {
		j.refuse("powershell-construct", "an argument to "+name, refusal)
		return
	}
	for _, reading := range readings {
		key := fmt.Sprintf("native %q %q %v", name, reading.Args, reading.ArgDynamic)
		if j.seen != nil {
			if j.seen[key] {
				continue
			}
			j.seen[key] = true
		}
		j.fail(j.s.addCall(name, false, reading.Args, reading.ArgDynamic, false, j.depth))
		j.pipedProgram(name, reading.Args, prev)
	}
}

// pipedProgram records the code a pipeline hands an interpreter on stdin, as addPipe does for bash: a string
// written into the pipe, or a file Get-Content reads into it.
func (j *psJudge) pipedProgram(name string, args []string, prev psparse.CommandBaseAst) {
	if prev == nil {
		return
	}
	lang := stdinLanguage(commandBase(name), args)
	if lang == "" {
		return
	}
	switch p := prev.(type) {
	case *psparse.CommandExpressionAst:
		for _, value := range j.values(p.Expression, psModern) {
			if value.Dynamic {
				continue
			}
			if lang == "sh" {
				j.fail(j.s.add(value.Text, j.depth+1))
				continue
			}
			j.s.addProgram(lang, value.Text, "", j.depth)
		}
	case *psparse.CommandAst:
		if len(p.CommandElements) == 0 {
			return
		}
		nameAst, ok := p.CommandElements[0].(*psparse.StringConstantExpressionAst)
		if !ok {
			return
		}
		modern, _, _ := j.elements(p.CommandElements[1:])
		for _, meaning := range psResolveName(nameAst.Value) {
			if meaning.Cmdlet != "get-content" {
				continue
			}
			bound := psBind("get-content", modern)
			for _, param := range []string{"path", "literalpath"} {
				for _, value := range bound.Params[param] {
					if !value.Dynamic {
						j.s.addProgram(lang, "", value.Text, j.depth)
					}
				}
			}
		}
	}
}

// record appends a call or redirect, once per command when the same record comes from several meanings.
func (j *psJudge) record(item any) {
	if j.seen != nil {
		key := fmt.Sprintf("%#v", item)
		if j.seen[key] {
			return
		}
		j.seen[key] = true
	}
	switch r := item.(type) {
	case shellCall:
		j.s.Calls = append(j.s.Calls, r)
	case shellRedirect:
		j.s.Redirects = append(j.s.Redirects, r)
	}
}
