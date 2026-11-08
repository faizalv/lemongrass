// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import "testing"

func TestTypeNames(t *testing.T) {
	str := &TypeName{TypeNameText: "string"}
	intT := &TypeName{TypeNameText: "int", Assembly: "mscorlib"}
	open := &TypeName{TypeNameText: "System.Collections.Generic.Dictionary", GenericArgumentCount: 2}
	dict := &GenericTypeName{TypeName: open, GenericArguments: []ITypeName{str, &ArrayTypeName{ElementType: intT, Rank: 2}}}
	cases := []struct {
		t              ITypeName
		fullName, name string
	}{
		{str, "string", "string"},
		{intT, "int,mscorlib", "int"},
		{&ArrayTypeName{ElementType: str, Rank: 1}, "string[]", "string[]"},
		{&ArrayTypeName{ElementType: intT, Rank: 3}, "int[,,],mscorlib", "int[,,]"},
		{dict, "System.Collections.Generic.Dictionary[string,int[,],mscorlib]", "System.Collections.Generic.Dictionary[string,int[,]]"},
		{&ArrayTypeName{ElementType: dict, Rank: 1}, "System.Collections.Generic.Dictionary[string,int[,]][]", "System.Collections.Generic.Dictionary[string,int[,]][]"},
		// Rank is at least 1 from the parser; a hand-built node with less must not panic.
		{&ArrayTypeName{ElementType: str}, "string[]", "string[]"},
	}
	for _, c := range cases {
		if got := c.t.FullName(); got != c.fullName {
			t.Errorf("FullName = %q, want %q", got, c.fullName)
		}
		if got := c.t.Name(); got != c.name {
			t.Errorf("Name = %q, want %q", got, c.name)
		}
	}
}

// The parser sets the open generic name's assembly after the wrappers exist (Parser.cs 1516, 1610), so the
// wrappers read it on demand.
func TestTypeNameAssemblyAfterConstruction(t *testing.T) {
	open := &TypeName{TypeNameText: "List", GenericArgumentCount: 1}
	generic := &GenericTypeName{TypeName: open, GenericArguments: []ITypeName{&TypeName{TypeNameText: "int"}}}
	array := &ArrayTypeName{ElementType: generic, Rank: 1}
	open.Assembly = "Lib"
	if got := generic.FullName(); got != "List[int],Lib" {
		t.Errorf("generic FullName = %q", got)
	}
	if got := array.FullName(); got != "List[int][],Lib" {
		t.Errorf("array FullName = %q", got)
	}
	if got := array.AssemblyName(); got != "Lib" {
		t.Errorf("array AssemblyName = %q", got)
	}
}
