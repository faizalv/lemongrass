// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import "strings"

// psNativeReading is one argv a native program can receive from a PowerShell command line.
type psNativeReading struct {
	Args       []string
	ArgDynamic []bool
}

// psNativeArg is one argument as PowerShell builds it for a native program, before it is passed.
type psNativeArg struct {
	Text    string
	Dynamic bool
	// Tilde marks an unquoted word starting with ~, which pwsh 7.6 on Windows and pwsh on Unix expand to the home
	// directory and 5.1 passes as written.
	Tilde bool
}

// psNativeArgv builds the argv a native program receives from the written elements under every way PowerShell passes
// arguments (5.1, pwsh Legacy, pwsh 7.3+ Windows and Standard). It refuses, with guidance, an argument the models pass
// differently: empty, holding a double quote, or holding whitespace with a trailing backslash. One reading takes ~ as
// written, another as the home directory where an edition expands it.
func psNativeArgv(program string, elems []psElement) (readings []psNativeReading, refusal string) {
	args, refusal := psNativeArgs(program, elems)
	if refusal != "" {
		return nil, refusal
	}
	home := psNativeReading{Args: []string{}, ArgDynamic: []bool{}}
	literal := psNativeReading{Args: []string{}, ArgDynamic: []bool{}}
	tilde := false
	for _, arg := range args {
		if !arg.Dynamic {
			if refusal := psNativeArgRefusal(program, arg.Text); refusal != "" {
				return nil, refusal
			}
		}
		home.Args = append(home.Args, arg.Text)
		home.ArgDynamic = append(home.ArgDynamic, arg.Dynamic)
		text := arg.Text
		if arg.Tilde {
			// 5.1 passes ~ as a name in the current directory; ./ keeps the bash-side checks from reading it as home.
			tilde = true
			text = "./" + text
		}
		literal.Args = append(literal.Args, text)
		literal.ArgDynamic = append(literal.ArgDynamic, arg.Dynamic)
	}
	if !tilde {
		return []psNativeReading{home}, ""
	}
	return []psNativeReading{home, literal}, ""
}

// psNativeArgs renders elements as the arguments PowerShell builds: -Name:value is one argument, an array literal is
// one argument joined with commas, and -- passes through.
func psNativeArgs(program string, elems []psElement) ([]psNativeArg, string) {
	var args []psNativeArg
	for _, elem := range elems {
		if elem.Splat {
			return nil, "Splatting into " + program + " passes arguments the guard cannot see. Write the arguments out."
		}
		if elem.Param == "-" {
			args = append(args, psNativeArg{Text: "--"})
			continue
		}
		text, dynamic := psJoinValues(elem.Values)
		if elem.Param != "" {
			if elem.Colon {
				args = append(args, psNativeArg{Text: "-" + elem.Param + ":" + text, Dynamic: dynamic})
				continue
			}
			args = append(args, psNativeArg{Text: "-" + elem.Param})
			if len(elem.Values) == 0 {
				continue
			}
		}
		tilde := len(elem.Values) == 1 && !elem.Values[0].Quoted && psExpandsTilde(text)
		args = append(args, psNativeArg{Text: text, Dynamic: dynamic, Tilde: tilde})
	}
	return args, ""
}

// psExpandsTilde reports whether pwsh expands a bare word's leading ~ for a native program: ~ alone or followed by a
// separator. ~user is never expanded.
func psExpandsTilde(text string) bool {
	return text == "~" || strings.HasPrefix(text, "~/") || strings.HasPrefix(text, `~\`)
}

// psNativeArgRefusal returns why an argument would reach the program differently under 5.1, pwsh Legacy and pwsh 7.3+:
// 5.1 and Legacy drop an empty argument and wrap one holding whitespace in quotes without escaping a quote inside, so
// the program splits it; 5.1 leaves a trailing backslash to escape the closing quote. An argument free of these
// reaches the program the same way under all three.
func psNativeArgRefusal(program, text string) string {
	switch {
	case text == "":
		return "Windows PowerShell drops an empty argument, so " + program + " would receive different arguments. Leave the empty argument out."
	case strings.ContainsAny(text, "\"“”„"):
		return "Windows PowerShell does not pass a double quote inside an argument to " + program + " intact. Use single quotes, or leave the quote out."
	case strings.ContainsAny(text, " \t") && strings.HasSuffix(text, `\`):
		return "Windows PowerShell does not pass an argument holding a space and ending in a backslash to " + program + " intact. Drop the trailing backslash."
	}
	return ""
}
