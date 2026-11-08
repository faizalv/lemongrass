// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import "strings"

// psValue is one value of a command argument, as the judge computed it from the tree.
type psValue struct {
	Text string
	// Dynamic marks a value not known at parse time.
	Dynamic bool
	// Quoted marks a value written quoted; ~ expansion and Unix globbing apply only to an unquoted bare word.
	Quoted bool
}

// psElement is one command element.
type psElement struct {
	// Param is a parameter element's name as written, without the dash, in any case and maybe abbreviated; "" for an argument.
	Param string
	// Colon marks -Name:value, whose value is in Values.
	Colon bool
	// Values holds an argument, one per array element (a,b is two); empty for a bare parameter.
	Values []psValue
	// Splat marks @x, which can bind any parameter.
	Splat bool
}

// psBound is a cmdlet call with its elements bound to parameters as PowerShell binds them.
type psBound struct {
	Cmdlet string
	// Params maps a canonical lower-case parameter name to its values.
	Params map[string][]psValue
	// Switches holds the switch parameters that are on; -Recurse:$false is off, a dynamic switch value counts as on.
	Switches map[string]bool
	// Refusal is guidance text when the binding cannot be known: a splat, or an unknown or ambiguous parameter on a table cmdlet.
	Refusal string
}

// psParam is one parameter of a modelled cmdlet. Position is its position in the default parameter set, or -1.
// Pipeline marks a parameter that can take its value from the pipeline.
type psParam struct {
	Name     string
	Aliases  []string
	Switch   bool
	Position int
	Pipeline bool
}

// psBind binds elements to the cmdlet's parameters: an exact name or alias first, else a unique prefix (an ambiguous
// one is PowerShell's own error), a switch takes no following value, and the rest bind by position. A cmdlet the
// table does not hold binds nothing and has no Refusal. A binding PowerShell itself rejects, such as an unknown
// parameter or a stray positional, is refused too: nothing runs then, and the refusal says what to write.
func psBind(cmdlet string, elems []psElement) psBound {
	bound := psBound{Cmdlet: cmdlet, Params: map[string][]psValue{}, Switches: map[string]bool{}}
	table, ok := psCmdletParams[cmdlet]
	if !ok {
		return bound
	}
	params := append(append([]psParam{}, table...), psCommonParams...)
	seen := map[string]bool{}
	var positional [][]psValue
	for i := 0; i < len(elems); i++ {
		elem := elems[i]
		switch {
		case elem.Splat:
			bound.Refusal = "Splatting into " + cmdlet + " binds parameters the guard cannot see. Write the parameters out."
			return bound
		case elem.Param == "-":
			// The first -- is ignored for a cmdlet; the parser already made every later word an argument.
			continue
		case elem.Param == "":
			positional = append(positional, elem.Values)
			continue
		}
		param, refusal := psLookupParam(cmdlet, params, elem.Param)
		if refusal != "" {
			bound.Refusal = refusal
			return bound
		}
		if seen[param.Name] {
			bound.Refusal = "-" + elem.Param + " is given twice to " + cmdlet + ", which PowerShell rejects. Give it once."
			return bound
		}
		seen[param.Name] = true
		if param.Switch {
			bound.Switches[param.Name] = !elem.Colon || psSwitchOn(elem.Values)
			continue
		}
		values := elem.Values
		if !elem.Colon {
			if i+1 >= len(elems) || elems[i+1].Param != "" || elems[i+1].Splat {
				bound.Refusal = "-" + elem.Param + " of " + cmdlet + " has no value, which PowerShell rejects. Give it a value."
				return bound
			}
			i++
			values = elems[i].Values
		}
		bound.Params[param.Name] = values
	}
	for _, values := range positional {
		param := psNextPositional(params, seen)
		if param == nil {
			text, _ := psJoinValues(values)
			bound.Refusal = cmdlet + " takes no further positional argument, so '" + text + "' would fail in PowerShell. Name its parameter."
			return bound
		}
		seen[param.Name] = true
		bound.Params[param.Name] = values
	}
	return bound
}

// psLookupParam finds the parameter a written name binds as PowerShell's MergedCommandParameterMetadata does: an exact
// name or alias, else the one parameter whose name or an alias starts with it.
func psLookupParam(cmdlet string, params []psParam, written string) (*psParam, string) {
	key := psFold(written)
	var match *psParam
	ambiguous := false
	for i := range params {
		param := &params[i]
		names := append([]string{param.Name}, param.Aliases...)
		for _, name := range names {
			if name == key {
				return param, ""
			}
		}
		for _, name := range names {
			if strings.HasPrefix(name, key) {
				if match != nil && match.Name != param.Name {
					ambiguous = true
				}
				match = param
				break
			}
		}
	}
	switch {
	case ambiguous:
		return nil, "-" + written + " is ambiguous for " + cmdlet + ", which PowerShell rejects. Write the full parameter name."
	case match == nil:
		return nil, cmdlet + " has no parameter -" + written + " that the guard knows. Write the full name of one of its parameters."
	}
	return match, ""
}

// psNextPositional returns the unbound parameter with the lowest position, or nil.
func psNextPositional(params []psParam, seen map[string]bool) *psParam {
	var next *psParam
	for i := range params {
		param := &params[i]
		if param.Position < 0 || seen[param.Name] {
			continue
		}
		if next == nil || param.Position < next.Position {
			next = param
		}
	}
	return next
}

// psSwitchOn reads a switch's colon value: off only for a known false value, so a dynamic one counts as on.
func psSwitchOn(values []psValue) bool {
	if len(values) != 1 || values[0].Dynamic {
		return true
	}
	switch strings.ToLower(values[0].Text) {
	case "false", "0", "", "$false":
		return false
	}
	return true
}

// psCmdletArgs renders a cmdlet call's elements as shellCall.Args with ArgDynamic parallel, for the forms that read
// cmdlet arguments with psParameter: a switch keeps -Name:value as one argument, any other -Name:value becomes -Name
// and its value so the path checks see the value alone, and each array element is its own argument.
func psCmdletArgs(cmdlet string, elems []psElement) (args []string, dynamic []bool) {
	for _, elem := range elems {
		if elem.Param != "" {
			if elem.Colon && psSwitchParam(cmdlet, elem.Param) {
				text, dyn := psJoinValues(elem.Values)
				args = append(args, "-"+elem.Param+":"+text)
				dynamic = append(dynamic, dyn)
				continue
			}
			args = append(args, "-"+elem.Param)
			dynamic = append(dynamic, false)
		}
		for _, value := range elem.Values {
			args = append(args, value.Text)
			dynamic = append(dynamic, value.Dynamic || elem.Splat)
		}
		if elem.Splat && len(elem.Values) == 0 {
			args = append(args, "@")
			dynamic = append(dynamic, true)
		}
	}
	return args, dynamic
}

// psSwitchParam reports whether name, written as on the command line, binds a switch parameter of cmdlet.
func psSwitchParam(cmdlet, name string) bool {
	table, ok := psCmdletParams[cmdlet]
	if !ok {
		return false
	}
	param, refusal := psLookupParam(cmdlet, append(append([]psParam{}, table...), psCommonParams...), name)
	return refusal == "" && param.Switch
}

func psJoinValues(values []psValue) (string, bool) {
	texts := make([]string, len(values))
	dynamic := false
	for i, value := range values {
		texts[i] = value.Text
		dynamic = dynamic || value.Dynamic
	}
	return strings.Join(texts, ","), dynamic
}
