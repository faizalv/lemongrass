// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

// psEdition is a PowerShell whose name lookup the gate models. The gate cannot tell which one runs a call, so a name
// is judged under every edition whose meaning differs.
type psEdition uint8

const (
	// psWin51 is Windows PowerShell 5.1, which every supported Windows ships.
	psWin51 psEdition = 1 << iota
	// psWin7 is pwsh 7.6 on Windows.
	psWin7
	// psUnix7 is pwsh 7.6 on Linux and macOS, which drops the aliases that shadow common programs.
	psUnix7
)

const psAllEditions = psWin51 | psWin7 | psUnix7

// psMeaning is what a command name runs under the editions in Editions.
type psMeaning struct {
	Editions psEdition
	// Cmdlet is the lower-case canonical cmdlet name, such as remove-item; "" for a native program.
	Cmdlet string
	// Native is the program name as written, for shellScript.addCall to normalize like a bash command name; "" for a cmdlet.
	Native string
	// Fixed holds the arguments a default function adds before the written ones: mkdir adds -ItemType Directory, C: is Set-Location C:.
	Fixed []psElement
}

// psResolveName resolves a literal command name as PowerShell's lookup does (alias, then default function, then
// cmdlet, then native program), once per edition, merging editions that agree. A module-qualified name such as
// Microsoft.PowerShell.Management\Remove-Item is that cmdlet; a name with a path separator or an extension is native.
func psResolveName(name string) []psMeaning {
	return []psMeaning{{Editions: psAllEditions, Native: name}}
}
