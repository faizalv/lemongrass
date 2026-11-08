// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

// The judge refuses what it cannot reason about rather than guessing: every refusal names the construct and says
// what to write instead, since the reader is a model that will retry.

const (
	psGuideMethod     = "PowerShell method calls and .NET static members, such as $x.Delete() or [IO.File]::Delete(), are not followed by the guard. Use a cmdlet or a program with the path written out, such as Remove-Item -LiteralPath <path>."
	psGuideInvokeExpr = "Invoke-Expression runs text the guard cannot see. Write the command itself."
	psGuideInvokeCmd  = "Invoke-Command runs a script block in another session or on another computer. Run the commands directly."
	psGuideJob        = "A background job runs in another process with its own working directory. Run the command in the foreground."
	psGuideModule     = "Loading a module, snap-in or assembly brings in commands the guard cannot see. Call the tool you need directly."
	psGuideDefinition = "A function, filter, class or enum definition makes later names run code the guard does not follow. Write the commands inline instead."
	psGuideInvokeItem = "Invoke-Item opens a file with its default program, which the guard cannot see. Run the program you mean with the file as its argument."
	psGuideStopParse  = "The stop-parsing token --% passes the rest of the line to the program unparsed. Write the arguments as quoted PowerShell strings instead."
	psGuideAlias      = "Redefining an alias or function changes what later commands run. Call the command by its own name."
	psGuideDrive      = "A new PowerShell drive makes later paths point somewhere the guard cannot see. Write the full path instead."
	psGuideHistory    = "Replaying a command from history runs a command the guard cannot see. Write the command itself."
	psGuideDynamic    = "The command name is computed at runtime. Write the command name out, such as & 'C:\\tools\\tool.exe' <arguments>."
	psGuideUnknown    = "This PowerShell construct is not modelled by the guard. Write it as plain commands with literal arguments."
)

// psRefusal is a rule and the guidance a refused construct carries.
type psRefusal struct {
	rule  string
	guide string
}

// psRefusedCmdlets are the cmdlets the judge refuses by canonical name, whatever alias reached them.
var psRefusedCmdlets = map[string]psRefusal{
	"invoke-expression": {"powershell-construct", psGuideInvokeExpr},
	"invoke-command":    {"powershell-construct", psGuideInvokeCmd},
	"start-job":         {"powershell-construct", psGuideJob},
	"import-module":     {"powershell-construct", psGuideModule},
	"add-pssnapin":      {"powershell-construct", psGuideModule},
	"add-type":          {"powershell-construct", psGuideModule},
	"invoke-item":       {"powershell-construct", psGuideInvokeItem},
	"set-alias":         {"name-redefinition", psGuideAlias},
	"new-alias":         {"name-redefinition", psGuideAlias},
	"new-psdrive":       {"drive-definition", psGuideDrive},
	"invoke-history":    {"history-replay", psGuideHistory},
}

// refuse records a refusal; what refused is named first so the model sees which part of its command it was.
func (j *psJudge) refuse(rule, what, guide string) {
	detail := guide
	if what != "" {
		detail = "Matched: " + what + ". " + guide
	}
	j.s.Refusals = append(j.s.Refusals, ruleVerdict(rule, detail))
}
