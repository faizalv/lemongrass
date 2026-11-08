// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

// ITypeName is a type as written in a type literal or constraint: [int], [List[string]], [byte[]]. Resolving it
// to a real type is out of scope; the judge reads only the name.
type ITypeName interface {
	FullName() string
	Name() string
	AssemblyName() string
	IsArray() bool
	IsGeneric() bool
	Extent() Extent
}

// TypeName is a simple or namespace-qualified type name.
type TypeName struct {
	Ext Extent
	// TypeNameText is the name as PowerShell stores it, with doubled backticks collapsed.
	TypeNameText string
	Assembly     string
	// GenericArgumentCount is set for the open generic name inside a GenericTypeName.
	GenericArgumentCount int
}

func (t *TypeName) FullName() string {
	if t.Assembly != "" {
		return t.TypeNameText + "," + t.Assembly
	}
	return t.TypeNameText
}
func (t *TypeName) Name() string         { return t.TypeNameText }
func (t *TypeName) AssemblyName() string { return t.Assembly }
func (t *TypeName) IsArray() bool        { return false }
func (t *TypeName) IsGeneric() bool      { return false }
func (t *TypeName) Extent() Extent       { return t.Ext }

// GenericTypeName is a generic type with its arguments, such as List[string].
type GenericTypeName struct {
	Ext              Extent
	TypeName         ITypeName
	GenericArguments []ITypeName
	// FullNameText and NameText are filled by the parser owner while porting GenericTypeName's name building.
	FullNameText string
	NameText     string
}

func (t *GenericTypeName) FullName() string     { return t.FullNameText }
func (t *GenericTypeName) Name() string         { return t.NameText }
func (t *GenericTypeName) AssemblyName() string { return t.TypeName.AssemblyName() }
func (t *GenericTypeName) IsArray() bool        { return false }
func (t *GenericTypeName) IsGeneric() bool      { return true }
func (t *GenericTypeName) Extent() Extent       { return t.Ext }

// ArrayTypeName is an array of an element type, such as byte[] or int[,].
type ArrayTypeName struct {
	Ext          Extent
	ElementType  ITypeName
	Rank         int
	FullNameText string
	NameText     string
}

func (t *ArrayTypeName) FullName() string     { return t.FullNameText }
func (t *ArrayTypeName) Name() string         { return t.NameText }
func (t *ArrayTypeName) AssemblyName() string { return t.ElementType.AssemblyName() }
func (t *ArrayTypeName) IsArray() bool        { return true }
func (t *ArrayTypeName) IsGeneric() bool      { return false }
func (t *ArrayTypeName) Extent() Extent       { return t.Ext }
