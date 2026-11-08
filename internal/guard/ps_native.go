// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

// psNativeReading is one argv a native program can receive from a PowerShell command line.
type psNativeReading struct {
	Args       []string
	ArgDynamic []bool
}

// psNativeArgv builds the argv a native program receives from the written elements under every way PowerShell passes
// arguments (5.1, pwsh Legacy, pwsh 7.3+ Windows and Standard). It refuses, with guidance, an argument the models pass
// differently: empty, holding a double quote, or holding whitespace with a trailing backslash. One reading takes ~ as
// written, another as the home directory where an edition expands it.
func psNativeArgv(program string, elems []psElement) (readings []psNativeReading, refusal string) {
	return nil, "Arguments to " + program + " from PowerShell are not judged yet."
}
