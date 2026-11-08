// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"regexp"
	"strings"

	"github.com/faizalv/lemongrass/internal/psparse"
)

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

var psEditionOrder = []psEdition{psWin51, psWin7, psUnix7}

// psMeaning is what a command name runs under the editions in Editions.
type psMeaning struct {
	Editions psEdition
	// Cmdlet is the lower-case name of the cmdlet or default function that runs, such as remove-item; "" for a native program.
	Cmdlet string
	// Native is the program name as written, for shellScript.addCall to normalize like a bash command name; "" for a cmdlet.
	Native string
	// Fixed holds the arguments a default function adds before the written ones: mkdir adds -ItemType Directory, C: is Set-Location C:.
	Fixed []psElement
}

type psAlias struct {
	Editions psEdition
	Target   string
}

// psFunction is a function every session defines, read as the cmdlet it calls with the arguments it adds.
type psFunction struct {
	Editions psEdition
	Cmdlet   string
	Fixed    []psElement
}

// psFunctions are InitialSessionState.BuiltInFunctions. Only the ones that change directory or write matter to the
// gate; the rest are named so they are not taken for native programs. exec is Unix only and replaces the shell with
// the command in its arguments (Switch-Process -WithCommand).
var psFunctions = map[string]psFunction{
	"cd..":          {psAllEditions, "set-location", []psElement{psFixedValue("..")}},
	`cd\`:           {psAllEditions, "set-location", []psElement{psFixedValue(`\`)}},
	"cd~":           {psWin7 | psUnix7, "set-location", []psElement{{Values: []psValue{{Text: "~"}}}}},
	"mkdir":         {psWin51 | psWin7, "new-item", []psElement{{Param: "ItemType", Values: []psValue{{Text: "Directory", Quoted: true}}}}},
	"help":          {psAllEditions, "get-help", nil},
	"pause":         {psAllEditions, "read-host", nil},
	"prompt":        {psAllEditions, "prompt", nil},
	"clear-host":    {psAllEditions, "clear-host", nil},
	"tabexpansion2": {psAllEditions, "tabexpansion2", nil},
	"oss":           {psAllEditions, "out-string", nil},
	"exec":          {psUnix7, "switch-process", nil},
}

// psDriveFunction matches the A: to Z: functions Windows sessions define, each a Set-Location to its drive.
var psDriveFunction = regexp.MustCompile(`^[A-Za-z]:$`)

// psModuleQualified matches Module\Verb-Noun, which names the command in that module; the same text can be a
// relative path to a program on Windows, so both meanings are judged.
var psModuleQualified = regexp.MustCompile(`^[A-Za-z0-9_.]+\\([A-Za-z]+-[A-Za-z0-9]+)$`)

func psFixedValue(text string) psElement {
	return psElement{Values: []psValue{{Text: text, Quoted: true}}}
}

// psResolveName resolves a literal command name as PowerShell's lookup does (alias, then default function, then
// cmdlet, then native program), once per edition, merging editions that agree. A module-qualified name such as
// Microsoft.PowerShell.Management\Remove-Item is that cmdlet or a relative program path; any other name with a path
// separator or a dot is native.
func psResolveName(name string) []psMeaning {
	if m := psModuleQualified.FindStringSubmatch(name); m != nil {
		return []psMeaning{
			{Editions: psAllEditions, Cmdlet: psFold(m[1])},
			{Editions: psAllEditions, Native: name},
		}
	}
	key := psFold(name)
	if _, ok := psFunctions[key]; !ok && strings.ContainsAny(name, `/\.`) {
		return []psMeaning{{Editions: psAllEditions, Native: name}}
	}
	var meanings []psMeaning
	for _, edition := range psEditionOrder {
		for _, meaning := range psResolveIn(name, edition) {
			meanings = psMergeMeaning(meanings, meaning)
		}
	}
	return meanings
}

// psResolveIn resolves name under one edition, following an alias to its target.
func psResolveIn(name string, edition psEdition) []psMeaning {
	key := psFold(name)
	if alias, ok := psAliasTable[key]; ok && alias.Editions&edition != 0 {
		name, key = alias.Target, psFold(alias.Target)
	}
	if fn, ok := psFunctions[key]; ok && fn.Editions&edition != 0 {
		return []psMeaning{{Editions: edition, Cmdlet: fn.Cmdlet, Fixed: fn.Fixed}}
	}
	if edition != psUnix7 && psDriveFunction.MatchString(name) {
		return []psMeaning{{Editions: edition, Cmdlet: "set-location", Fixed: []psElement{psFixedValue(name)}}}
	}
	if _, ok := psCmdletNames[key]; ok {
		return []psMeaning{{Editions: edition, Cmdlet: key}}
	}
	if strings.ContainsAny(name, `/\.`) {
		return []psMeaning{{Editions: edition, Native: name}}
	}
	// A Verb-Noun name with an approved verb may be a cmdlet or function a module loads on first use, or a program.
	if dash := strings.IndexByte(key, '-'); dash > 0 && psApprovedVerbs[key[:dash]] {
		return []psMeaning{{Editions: edition, Cmdlet: key}, {Editions: edition, Native: name}}
	}
	return []psMeaning{{Editions: edition, Native: name}}
}

func psMergeMeaning(meanings []psMeaning, next psMeaning) []psMeaning {
	for i, meaning := range meanings {
		if meaning.Cmdlet == next.Cmdlet && meaning.Native == next.Native && psSameElements(meaning.Fixed, next.Fixed) {
			meanings[i].Editions |= next.Editions
			return meanings
		}
	}
	return append(meanings, next)
}

func psSameElements(a, b []psElement) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Param != b[i].Param || a[i].Colon != b[i].Colon || a[i].Splat != b[i].Splat || len(a[i].Values) != len(b[i].Values) {
			return false
		}
		for j := range a[i].Values {
			if a[i].Values[j] != b[i].Values[j] {
				return false
			}
		}
	}
	return true
}

// psFold is the key PowerShell compares command and parameter names by (OrdinalIgnoreCase), lowered so the tables
// read as written in PowerShell's own sources.
func psFold(name string) string {
	return strings.ToLower(psparse.OrdinalUpper(name))
}

// psApprovedVerbs are the verbs Get-Verb lists; a module command is named with one of them.
var psApprovedVerbs = func() map[string]bool {
	verbs := map[string]bool{}
	for _, verb := range strings.Fields(`add clear close copy enter exit find format get hide join lock move new open
		optimize pop push redo remove rename reset resize search select set show skip split step switch undo unlock
		watch connect disconnect read receive send write backup checkpoint compare compress convert convertfrom
		convertto dismount edit expand export group import initialize limit merge mount out publish restore save
		sync unpublish update debug measure ping repair resolve test trace approve assert build complete confirm deny
		deploy disable enable install invoke register request restart resume start stop submit suspend uninstall
		unregister wait block grant protect revoke unblock unprotect use`) {
		verbs[verb] = true
	}
	return verbs
}()
