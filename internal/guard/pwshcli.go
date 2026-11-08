// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"strings"
)

// pwshHosts are the PowerShell executables whose command line is read. powershell is Windows PowerShell 5.1, which
// reads its first positional argument as the start of the command text; pwsh (6 and later) reads it as a script file.
var pwshHosts = map[string]bool{"pwsh": true, "powershell": true, "pwsh-preview": true, "pwsh-lts": true}

type pwshParamKind int

const (
	pwshSwitch pwshParamKind = iota
	pwshValue
	pwshCommand
	pwshCommandWithArgs
	pwshFile
	pwshEncoded
	pwshWorkingDirectory
	pwshVersion
	pwshHelp
)

// pwshParam is one host parameter. A written name matches when it is a prefix of name at least as long as short, the
// way the hosts' MatchSwitch abbreviates, or equals one of aliases.
type pwshParam struct {
	name, short string
	aliases     []string
	kind        pwshParamKind
}

// pwshParams are the parameters of pwsh 7 and Windows PowerShell 5.1 the guard reads. An abbreviation that fits two
// of them is refused rather than resolved, so the guard never depends on the order a host checks them in.
var pwshParams = []pwshParam{
	{name: "help", short: "h", aliases: []string{"?"}, kind: pwshHelp},
	{name: "login", short: "l", kind: pwshSwitch},
	{name: "noexit", short: "noe", kind: pwshSwitch},
	{name: "noprofile", short: "nop", kind: pwshSwitch},
	{name: "noprofileloadtime", short: "noprofileloadtime", kind: pwshSwitch},
	{name: "nologo", short: "nol", kind: pwshSwitch},
	{name: "noninteractive", short: "noni", kind: pwshSwitch},
	{name: "interactive", short: "i", kind: pwshSwitch},
	{name: "sta", short: "s", kind: pwshSwitch},
	{name: "mta", short: "mta", kind: pwshSwitch},
	{name: "version", short: "v", kind: pwshVersion},
	{name: "windowstyle", short: "w", kind: pwshValue},
	{name: "workingdirectory", short: "wo", aliases: []string{"wd"}, kind: pwshWorkingDirectory},
	{name: "executionpolicy", short: "ex", aliases: []string{"ep"}, kind: pwshValue},
	{name: "inputformat", short: "in", aliases: []string{"if"}, kind: pwshValue},
	{name: "outputformat", short: "o", aliases: []string{"of"}, kind: pwshValue},
	{name: "configurationname", short: "config", kind: pwshValue},
	{name: "settingsfile", short: "settings", kind: pwshValue},
	{name: "encodedarguments", short: "encodeda", aliases: []string{"ea"}, kind: pwshEncoded},
	{name: "encodedcommand", short: "e", aliases: []string{"ec"}, kind: pwshEncoded},
	{name: "commandwithargs", short: "commandwithargs", aliases: []string{"cwa"}, kind: pwshCommandWithArgs},
	{name: "command", short: "c", kind: pwshCommand},
	{name: "file", short: "f", kind: pwshFile},
}

// pwshParamOf reads arg as a host parameter. ok is false for a positional argument. A dash may be written as -, --,
// or the en dash, em dash or horizontal bar PowerShell reads as one; a / prefix is a parameter on Windows and a path
// elsewhere, so a /name that names a parameter is ambiguous and refused.
func pwshParamOf(arg string) (param pwshParam, ok, ambiguous, unknown bool) {
	var name string
	switch {
	case strings.HasPrefix(arg, "--"):
		name = arg[2:]
	case strings.HasPrefix(arg, "-"):
		name = arg[1:]
	case strings.HasPrefix(arg, "–"), strings.HasPrefix(arg, "—"), strings.HasPrefix(arg, "―"):
		name = arg[len("–"):]
	case arg == "/?":
		return pwshParams[0], true, false, false
	case strings.HasPrefix(arg, "/") && len(arg) > 1:
		if _, matches := pwshMatch(strings.ToLower(arg[1:])); matches > 0 {
			return pwshParam{}, false, true, false
		}
		return pwshParam{}, false, false, false
	default:
		return pwshParam{}, false, false, false
	}
	p, matches := pwshMatch(strings.ToLower(name))
	switch matches {
	case 0:
		return pwshParam{}, false, false, true
	case 1:
		return p, true, false, false
	}
	return pwshParam{}, false, true, false
}

// pwshMatch returns the parameter name spells and how many parameters it fits; an exact name or alias fits only its own.
func pwshMatch(name string) (pwshParam, int) {
	if name == "" {
		return pwshParam{}, 0
	}
	var found pwshParam
	matches := 0
	for _, p := range pwshParams {
		if name == p.name {
			return p, 1
		}
		for _, alias := range p.aliases {
			if name == alias {
				return p, 1
			}
		}
		if strings.HasPrefix(p.name, name) && strings.HasPrefix(name, p.short) {
			found = p
			matches++
		}
	}
	return found, matches
}

// addPwshCall reads the command line of a PowerShell host the way the host does and judges what it runs: -Command
// text through the PowerShell judge, as bash -c text goes through the shell parser. What the guard cannot read is
// refused with what to write instead.
func (s *shellScript) addPwshCall(host string, args []string, argDynamic []bool, depth int) error {
	refuse := func(rule, detail string) error {
		s.Refusals = append(s.Refusals, ruleVerdict(rule, "Matched: "+host+" "+detail))
		return nil
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		param, isParam, ambiguous, unknown := pwshParamOf(arg)
		switch {
		case ambiguous && strings.HasPrefix(arg, "/"):
			return refuse("opaque-shell", "with "+arg+", which Windows reads as a parameter and other systems as a path. Write PowerShell parameters with -.")
		case ambiguous:
			return refuse("opaque-shell", "with the parameter "+arg+", which abbreviates more than one parameter. Write it out in full, such as -NoProfile or -Command.")
		case unknown:
			return refuse("opaque-shell", "with the parameter "+arg+", which the guard does not read. Write it out in full, such as -NoProfile or -Command.")
		case !isParam:
			if host == "powershell" {
				return s.addPwshCommand(host, args[i:], sliceFlags(argDynamic, i, len(args)), depth, refuse)
			}
			return s.addPwshFile(host, arg, depth, refuse)
		}
		switch param.kind {
		case pwshHelp:
			return nil
		case pwshVersion:
			if host != "powershell" {
				return nil
			}
			// Windows PowerShell's -Version picks the engine version to start and takes a value.
			fallthrough
		case pwshValue, pwshWorkingDirectory:
			if i+1 >= len(args) {
				// A parameter missing its value is an error the host stops on, running nothing.
				return nil
			}
			if param.kind == pwshWorkingDirectory {
				// The host changes directory before the command runs, as a cd would, so the deletion scope follows it.
				s.Calls = append(s.Calls, shellCall{Name: "cd", Path: "cd", Args: []string{args[i+1]}, ArgDynamic: []bool{dynamicAt(argDynamic, i+1)}})
			}
			i++
		case pwshEncoded:
			return refuse("powershell-construct", "with "+arg+", which hides the command text. Pass the command as text with -Command instead.")
		case pwshFile:
			if i+1 >= len(args) {
				return nil
			}
			return s.addPwshFile(host, args[i+1], depth, refuse)
		case pwshCommandWithArgs:
			// The first value is the command; the rest reach it as $args, which the judge reads as values not known
			// at parse time.
			if i+1 >= len(args) {
				return nil
			}
			return s.addPwshCommand(host, args[i+1:i+2], sliceFlags(argDynamic, i+1, i+2), depth, refuse)
		case pwshCommand:
			if i+1 >= len(args) {
				return nil
			}
			return s.addPwshCommand(host, args[i+1:], sliceFlags(argDynamic, i+1, len(args)), depth, refuse)
		}
	}
	return refuse("opaque-shell", "with no command, which starts an interactive PowerShell the guard cannot see. Pass the command with -Command.")
}

// addPwshFile records the script a host runs, read at judge time like a shell script; the words after it are the
// script's arguments.
func (s *shellScript) addPwshFile(host, file string, depth int, refuse func(rule, detail string) error) error {
	if file == "-" {
		return refuse("opaque-shell", "reading its commands from standard input, which the guard cannot see. Pass the command with -Command.")
	}
	s.addProgram("powershell", "", file, depth)
	return nil
}

// addPwshCommand judges the words after -Command, which the host joins with spaces into one command text.
func (s *shellScript) addPwshCommand(host string, words []string, dynamic []bool, depth int, refuse func(rule, detail string) error) error {
	for i := range words {
		if dynamicAt(dynamic, i) {
			return refuse("dynamic-command", "with command text computed at runtime, so what it runs cannot be checked. Write the command out.")
		}
	}
	text := strings.Join(words, " ")
	if strings.TrimSpace(text) == "-" {
		return refuse("opaque-shell", "reading its commands from standard input, which the guard cannot see. Pass the command with -Command.")
	}
	return s.addPowerShell(text, depth+1)
}
