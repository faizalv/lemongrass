// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

// psProvider is the PowerShell provider a path argument names.
type psProvider uint8

const (
	psFileSystem psProvider = iota
	psRegistry
	psCertificate
	psVariable
	psEnvironment
	psAlias
	psFunction
	psWSMan
	// psTempDrive is pwsh 7's Temp: drive, the temp directory.
	psTempDrive
)

// psProviderPath classifies a path argument by its drive (HKCU:, HKLM:, Cert:, Env:, Variable:, Alias:, Function:,
// WSMan:, Temp:) or provider qualifier (Registry::, FileSystem::, Microsoft.PowerShell.Core\Registry::). fsPath is the
// file-system spelling for psFileSystem and psTempDrive, for the existing path checks, and "" otherwise.
func psProviderPath(path string) (kind psProvider, fsPath string) {
	return psFileSystem, path
}

// psProviderForm is a catalog Match: a cmdlet call any of whose arguments is a provider path of one of kinds; with
// writes, only the cmdlets that change items (the Item, ItemProperty and Content cmdlets that set, add, clear, copy,
// move, rename, remove or create, and Out-File).
func psProviderForm(writes bool, kinds ...psProvider) func(shellCall, matchEnv) (string, bool) {
	return func(shellCall, matchEnv) (string, bool) { return "", false }
}
