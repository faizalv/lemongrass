// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"strings"

	"github.com/faizalv/lemongrass/internal/psparse"
)

// powerShellCodeRunners are PowerShell commands that run code or another program given to them, so their call is
// never filed whole, like codeRunners for bash. Names are compared lower case, aliases included.
var powerShellCodeRunners = map[string]bool{
	"pwsh": true, "powershell": true, "invoke-expression": true, "iex": true, "invoke-command": true, "icm": true,
	"start-process": true, "start": true, "saps": true, "start-job": true, "sajb": true, "invoke-item": true, "ii": true,
	"foreach-object": true, "%": true, "where-object": true, "?": true, "cmd": true, "switch-process": true, "exec": true,
}

// simplePowerShellCommand is simpleCommand for PowerShell text: one command with a literal name whose arguments are
// constants, parameters and variables, nothing that runs code of its own.
func simplePowerShellCommand(command string) (string, bool) {
	res, err := psparse.Parse(command, psparse.DefaultLimits)
	if err != nil {
		return "PowerShell text that cannot be parsed", false
	}
	sb := res.Ast
	if sb.ParamBlock != nil || sb.BeginBlock != nil || sb.ProcessBlock != nil || sb.CleanBlock != nil || sb.DynamicParamBlock != nil || len(sb.UsingStatements) > 0 {
		return "a PowerShell script with its own blocks", false
	}
	if req := sb.ScriptRequirements; req != nil && (len(req.RequiredModules) > 0 || len(req.RequiredAssemblies) > 0 || req.RequiredPSVersion != "" || len(req.RequiredPSEditions) > 0 || req.IsElevationRequired || req.RequiredApplicationID != "") {
		return "a #requires line", false
	}
	if sb.EndBlock == nil || len(sb.EndBlock.Statements) != 1 || len(sb.EndBlock.Traps) > 0 {
		return "several commands", false
	}
	pipeline, ok := sb.EndBlock.Statements[0].(*psparse.PipelineAst)
	if !ok {
		return powerShellShapeOf(sb.EndBlock.Statements[0]), false
	}
	if pipeline.Background {
		return "a command sent to the background", false
	}
	if len(pipeline.PipelineElements) != 1 {
		return "a pipeline of several commands", false
	}
	cmd, ok := pipeline.PipelineElements[0].(*psparse.CommandAst)
	if !ok {
		return "an expression rather than a command", false
	}
	name, ok := cmd.GetCommandName()
	if !ok {
		return "a command name computed at run time", false
	}
	base := strings.ToLower(runnerName(strings.ReplaceAll(name, `\`, "/")))
	base = strings.TrimSuffix(base, ".exe")
	if powerShellCodeRunners[base] || codeRunners[base] {
		return "code run by " + base, false
	}
	for _, element := range cmd.CommandElements[1:] {
		if !plainPowerShellElement(element) {
			return "an argument that runs code or computes a value", false
		}
	}
	for _, redirection := range cmd.Redirections {
		if file, ok := redirection.(*psparse.FileRedirectionAst); ok && !plainPowerShellElement(file.Location) {
			return "a redirection to a computed target", false
		}
	}
	return "", true
}

// plainPowerShellElement reports an argument that is only text: a constant, a parameter, a variable, a string over
// variables, or a list of these.
func plainPowerShellElement(element psparse.Ast) bool {
	switch e := element.(type) {
	case *psparse.StringConstantExpressionAst, *psparse.ConstantExpressionAst:
		return true
	case *psparse.VariableExpressionAst:
		return !e.Splatted
	case *psparse.CommandParameterAst:
		return e.Argument == nil || plainPowerShellElement(e.Argument)
	case *psparse.ExpandableStringExpressionAst:
		for _, nested := range e.NestedExpressions {
			if v, ok := nested.(*psparse.VariableExpressionAst); !ok || v.Splatted {
				return false
			}
		}
		return true
	case *psparse.ArrayLiteralAst:
		for _, item := range e.Elements {
			if !plainPowerShellElement(item) {
				return false
			}
		}
		return true
	}
	return false
}

func powerShellShapeOf(statement psparse.StatementAst) string {
	switch statement.(type) {
	case *psparse.PipelineChainAst:
		return "several commands chained with && or ||"
	case *psparse.AssignmentStatementAst:
		return "a variable assignment"
	case *psparse.FunctionDefinitionAst:
		return "a function definition"
	}
	return "PowerShell control flow or a block of statements"
}
