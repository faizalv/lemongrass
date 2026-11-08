// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// VariablePath is PowerShell's VariablePath (engine/VariablePath.cs): a variable name with its optional scope or
// drive qualifier, as in $env:PATH, $global:x or ${C:\file.txt}.
type VariablePath struct {
	// UserPath is the path as written, without the sigil.
	UserPath string
	// The fields below are the tokenizer owner's to define while porting VariablePath.cs.
}

// NewVariablePath parses path the way VariablePath's constructor does.
func NewVariablePath(path string) *VariablePath {
	panic(notPorted)
}

// UnqualifiedPath is the name without scope or drive qualifier.
func (v *VariablePath) UnqualifiedPath() string { panic(notPorted) }

// DriveName is the drive qualifier ("env" for $env:PATH), empty when there is none.
func (v *VariablePath) DriveName() string { panic(notPorted) }

// IsDriveQualified reports whether the path names a provider drive.
func (v *VariablePath) IsDriveQualified() bool { panic(notPorted) }

// IsVariable reports whether the path names a variable rather than a drive item.
func (v *VariablePath) IsVariable() bool { panic(notPorted) }

// IsGlobal, IsScript, IsLocal and IsPrivate report the scope qualifier.
func (v *VariablePath) IsGlobal() bool  { panic(notPorted) }
func (v *VariablePath) IsScript() bool  { panic(notPorted) }
func (v *VariablePath) IsLocal() bool   { panic(notPorted) }
func (v *VariablePath) IsPrivate() bool { panic(notPorted) }

// notPorted marks a contract stub whose body the owning thinker ports.
const notPorted = "psparse: not ported yet"
