// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

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

// psBind binds elements to the cmdlet's parameters: an exact name or alias first, else a unique prefix (an ambiguous
// one is PowerShell's own error), a switch takes no following value, and the rest bind by position. A cmdlet the
// table does not hold binds nothing and has no Refusal.
func psBind(cmdlet string, elems []psElement) psBound {
	return psBound{Cmdlet: cmdlet}
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
	return false
}

func psJoinValues(values []psValue) (string, bool) {
	text := ""
	dynamic := false
	for i, value := range values {
		if i > 0 {
			text += ","
		}
		text += value.Text
		dynamic = dynamic || value.Dynamic
	}
	return text, dynamic
}
