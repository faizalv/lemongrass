// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors
// Derived from PowerShell (github.com/PowerShell/PowerShell), Copyright (c) Microsoft Corporation, MIT; see LICENSE.PowerShell.

package psparse

import "strings"

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
}

// FullName and Name are computed on read, as in C#, because the parser sets the open name's assembly after the
// wrapper is built.
func (t *GenericTypeName) FullName() string {
	s := t.genericName(func(a ITypeName) string { return a.FullName() })
	if asm := t.TypeName.AssemblyName(); asm != "" {
		s += "," + asm
	}
	return s
}
func (t *GenericTypeName) Name() string {
	return t.genericName(func(a ITypeName) string { return a.Name() })
}

func (t *GenericTypeName) genericName(arg func(ITypeName) string) string {
	var b strings.Builder
	b.WriteString(t.TypeName.Name())
	b.WriteByte('[')
	for i, a := range t.GenericArguments {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(arg(a))
	}
	b.WriteByte(']')
	return b.String()
}

func (t *GenericTypeName) AssemblyName() string { return t.TypeName.AssemblyName() }
func (t *GenericTypeName) IsArray() bool        { return false }
func (t *GenericTypeName) IsGeneric() bool      { return true }
func (t *GenericTypeName) Extent() Extent       { return t.Ext }

// ArrayTypeName is an array of an element type, such as byte[] or int[,].
type ArrayTypeName struct {
	Ext         Extent
	ElementType ITypeName
	Rank        int
}

// FullName and Name are computed on read, as in C# (GetName).
func (t *ArrayTypeName) FullName() string {
	s := t.Name()
	if asm := t.ElementType.AssemblyName(); asm != "" {
		s += "," + asm
	}
	return s
}
func (t *ArrayTypeName) Name() string {
	commas := ""
	if t.Rank > 1 {
		commas = strings.Repeat(",", t.Rank-1)
	}
	return t.ElementType.Name() + "[" + commas + "]"
}
func (t *ArrayTypeName) AssemblyName() string { return t.ElementType.AssemblyName() }
func (t *ArrayTypeName) IsArray() bool        { return true }
func (t *ArrayTypeName) IsGeneric() bool      { return false }
func (t *ArrayTypeName) Extent() Extent       { return t.Ext }
