// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import "strings"

// variablePathFlags is VariablePathFlags.
type variablePathFlags uint8

const (
	vpNone           variablePathFlags = 0x00
	vpLocal          variablePathFlags = 0x01
	vpScript         variablePathFlags = 0x02
	vpGlobal         variablePathFlags = 0x04
	vpPrivate        variablePathFlags = 0x08
	vpVariable       variablePathFlags = 0x10
	vpFunction       variablePathFlags = 0x20
	vpDriveQualified variablePathFlags = 0x40
	vpUnqualified    variablePathFlags = 0x80

	vpUnscopedVariableMask = vpLocal | vpScript | vpGlobal | vpPrivate | vpFunction | vpDriveQualified
)

// VariablePath is PowerShell's VariablePath (engine/VariablePath.cs): a variable name with its optional scope or
// drive qualifier, as in $env:PATH, $global:x or ${C:\file.txt}.
type VariablePath struct {
	// UserPath is the path as written, without the sigil.
	UserPath    string
	unqualified string
	flags       variablePathFlags
}

// NewVariablePath parses path the way VariablePath's constructor does. C# throws on an empty path; the tokenizer
// never builds one, and here it yields an unqualified variable rather than a panic.
func NewVariablePath(path string) *VariablePath { return newVariablePathFlags(path, vpNone) }

// newVariablePathFlags is VariablePath(string, VariablePathFlags). The path is scanned by byte: every character
// it tests is ASCII, and a UTF-8 byte of a non-ASCII character never equals one.
func newVariablePathFlags(path string, knownFlags variablePathFlags) *VariablePath {
	v := &VariablePath{UserPath: path, flags: knownFlags}
	if path == "" {
		v.flags = vpUnqualified | vpVariable
		return v
	}
	cur := 0
	lastColon := -1
	var scope string
	var scopeFlags variablePathFlags
	// C# rescans after variable: by jumping back to a switch on path[0], which is then 'v' with knownFlags set,
	// so no scope is ever read after variable: and variable:global:x keeps global:x as its name.
	switch path[0] {
	case 'g', 'G':
		scope, scopeFlags = "LOBAL", vpGlobal
	case 'l', 'L':
		scope, scopeFlags = "OCAL", vpLocal
	case 'p', 'P':
		scope, scopeFlags = "RIVATE", vpPrivate
	case 's', 'S':
		scope, scopeFlags = "CRIPT", vpScript
	case 'v', 'V':
		if knownFlags == vpNone {
			scope, scopeFlags = "ARIABLE", vpVariable
		}
	}
	if scope != "" {
		cur++
		j := 0
		for ; cur < len(path) && j < len(scope); j, cur = j+1, cur+1 {
			c := path[cur]
			if c != scope[j] && c != scope[j]+('a'-'A') {
				break
			}
		}
		if j == len(scope) && cur < len(path) && path[cur] == ':' {
			if v.flags == vpNone {
				v.flags = vpVariable
			}
			v.flags |= scopeFlags
			lastColon = cur
			cur++
		}
	}
	if v.flags == vpNone {
		lastColon = -1
		if i := strings.IndexByte(path[cur:], ':'); i >= 0 {
			lastColon = cur + i
		}
		if lastColon > 0 {
			v.flags = vpDriveQualified
		}
	}
	if lastColon == -1 {
		v.unqualified = path
	} else {
		v.unqualified = path[lastColon+1:]
	}
	if v.flags == vpNone {
		v.flags = vpUnqualified | vpVariable
	}
	return v
}

// cloneAndSetLocal is CloneAndSetLocal.
func (v *VariablePath) cloneAndSetLocal() *VariablePath {
	return &VariablePath{UserPath: v.UserPath, unqualified: v.unqualified, flags: vpLocal | vpVariable}
}

// UnqualifiedPath is the name without scope or drive qualifier.
func (v *VariablePath) UnqualifiedPath() string { return v.unqualified }

// QualifiedName is QualifiedName: the user path for a drive-qualified path, else the unqualified one.
func (v *VariablePath) QualifiedName() string {
	if v.IsDriveQualified() {
		return v.UserPath
	}
	return v.unqualified
}

// DriveName is the drive qualifier ("env" for $env:PATH), empty when there is none (null in C#).
func (v *VariablePath) DriveName() string {
	if !v.IsDriveQualified() {
		return ""
	}
	return v.UserPath[:strings.IndexByte(v.UserPath, ':')]
}

// IsDriveQualified reports whether the path names a provider drive.
func (v *VariablePath) IsDriveQualified() bool { return v.flags&vpDriveQualified != 0 }

// IsVariable reports whether the path names a variable rather than a drive item.
func (v *VariablePath) IsVariable() bool { return v.flags&vpVariable != 0 }

// IsUnqualified is IsUnqualified.
func (v *VariablePath) IsUnqualified() bool { return v.flags&vpUnqualified != 0 }

// IsUnscopedVariable is IsUnscopedVariable.
func (v *VariablePath) IsUnscopedVariable() bool { return v.flags&vpUnscopedVariableMask == 0 }

// IsGlobal, IsScript, IsLocal and IsPrivate report the scope qualifier.
func (v *VariablePath) IsGlobal() bool  { return v.flags&vpGlobal != 0 }
func (v *VariablePath) IsScript() bool  { return v.flags&vpScript != 0 }
func (v *VariablePath) IsLocal() bool   { return v.flags&vpLocal != 0 }
func (v *VariablePath) IsPrivate() bool { return v.flags&vpPrivate != 0 }

// notPorted marks a contract stub whose body the owning thinker ports.
const notPorted = "psparse: not ported yet"
