// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import "testing"

func TestCommandAstGetCommandName(t *testing.T) {
	str := func(v string) *StringConstantExpressionAst { return &StringConstantExpressionAst{Value: v} }
	cases := []struct {
		elems []CommandElementAst
		name  string
		ok    bool
	}{
		{[]CommandElementAst{str("Get-Item"), str("x")}, "Get-Item", true},
		{[]CommandElementAst{str("")}, "", true},
		{[]CommandElementAst{&VariableExpressionAst{}}, "", false},
		{[]CommandElementAst{&ExpandableStringExpressionAst{Value: "a"}}, "", false},
		{nil, "", false},
	}
	for i, c := range cases {
		name, ok := (&CommandAst{CommandElements: c.elems}).GetCommandName()
		if name != c.name || ok != c.ok {
			t.Errorf("case %d: GetCommandName = %q, %v; want %q, %v", i, name, ok, c.name, c.ok)
		}
	}
}
