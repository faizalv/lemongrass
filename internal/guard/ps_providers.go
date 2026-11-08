// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// psProvider is the PowerShell provider a path argument names.
type psProvider uint8

const (
	psProviderFileSystem psProvider = iota
	psProviderRegistry
	psProviderCertificate
	psProviderVariable
	psProviderEnvironment
	psProviderAlias
	psProviderFunction
	psProviderWSMan
	// psProviderTemp is pwsh 7's Temp: drive, the temp directory.
	psProviderTemp
	// psProviderUnknown is a drive or provider no default session defines, such as one a profile or New-PSDrive adds.
	psProviderUnknown
)

var psProviderNames = map[string]psProvider{
	"filesystem": psProviderFileSystem, "registry": psProviderRegistry, "certificate": psProviderCertificate,
	"variable": psProviderVariable, "environment": psProviderEnvironment, "alias": psProviderAlias,
	"function": psProviderFunction, "wsman": psProviderWSMan,
}

// psDriveNames are the drives of a default session besides the single-letter file-system drives.
var psDriveNames = map[string]psProvider{
	"hkcu": psProviderRegistry, "hklm": psProviderRegistry, "cert": psProviderCertificate, "variable": psProviderVariable,
	"env": psProviderEnvironment, "alias": psProviderAlias, "function": psProviderFunction, "wsman": psProviderWSMan,
	"temp": psProviderTemp,
}

// A provider-qualified path is [Module\]Provider::path; a drive-qualified one is Drive:path.
var (
	psProviderQualified = regexp.MustCompile(`^(?:[A-Za-z0-9_.]+\\)?([A-Za-z0-9_]+)::(.*)$`)
	psDriveQualified    = regexp.MustCompile(`^([A-Za-z0-9_]+):(.*)$`)
)

// psProviderPath classifies a path argument by its drive (HKCU:, HKLM:, Cert:, Env:, Variable:, Alias:, Function:,
// WSMan:, Temp:) or provider qualifier (Registry::, FileSystem::, Microsoft.PowerShell.Core\Registry::). fsPath is the
// file-system spelling for psProviderFileSystem and psProviderTemp, for the existing path checks, and "" otherwise.
func psProviderPath(path string) (kind psProvider, fsPath string) {
	if m := psProviderQualified.FindStringSubmatch(path); m != nil {
		kind, ok := psProviderNames[strings.ToLower(m[1])]
		switch {
		case !ok:
			return psProviderUnknown, ""
		case kind == psProviderFileSystem:
			return kind, m[2]
		}
		return kind, ""
	}
	m := psDriveQualified.FindStringSubmatch(path)
	if m == nil || len(m[1]) == 1 {
		return psProviderFileSystem, path
	}
	kind, ok := psDriveNames[strings.ToLower(m[1])]
	switch {
	case !ok:
		return psProviderUnknown, ""
	case kind == psProviderTemp:
		// Temp: is the directory Path.GetTempPath returns, which os.TempDir reads from the same variables.
		rest := strings.TrimLeft(m[2], `/\`)
		if rest == "" {
			return kind, os.TempDir()
		}
		return kind, filepath.Join(os.TempDir(), rest)
	}
	return kind, ""
}

// psWriteCmdlets change the item a path names; psProviderForm with writes matches only them.
var psWriteCmdlets = map[string]bool{
	"set-item": true, "new-item": true, "remove-item": true, "rename-item": true, "copy-item": true, "move-item": true,
	"clear-item": true, "set-itemproperty": true, "new-itemproperty": true, "remove-itemproperty": true,
	"rename-itemproperty": true, "copy-itemproperty": true, "move-itemproperty": true, "clear-itemproperty": true,
	"set-content": true, "add-content": true, "clear-content": true, "out-file": true, "tee-object": true,
	"export-csv": true, "export-clixml": true, "set-acl": true,
}

// psProviderForm is a catalog Match: a cmdlet call any of whose arguments is a provider path of one of kinds; with
// writes, only the cmdlets that change items. It reads the arguments psCmdletArgs renders, where a parameter's value
// is its own argument.
func psProviderForm(writes bool, kinds ...psProvider) func(shellCall, matchEnv) (string, bool) {
	return func(call shellCall, _ matchEnv) (string, bool) {
		if writes && !psWriteCmdlets[call.Name] {
			return "", false
		}
		for i, arg := range call.Args {
			if strings.HasPrefix(arg, "-") || dynamicAt(call.ArgDynamic, i) {
				continue
			}
			kind, _ := psProviderPath(arg)
			for _, want := range kinds {
				if kind == want {
					return "Matched: " + call.Name + " on " + arg + ".", true
				}
			}
		}
		return "", false
	}
}
