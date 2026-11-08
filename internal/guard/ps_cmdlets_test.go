// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"reflect"
	"strings"
	"testing"
)

func psArg(texts ...string) psElement {
	elem := psElement{}
	for _, text := range texts {
		elem.Values = append(elem.Values, psValue{Text: text})
	}
	return elem
}

func psParamElem(name string, texts ...string) psElement {
	elem := psArg(texts...)
	elem.Param = name
	elem.Colon = len(texts) > 0
	return elem
}

func TestPsBind(t *testing.T) {
	b := psBind("remove-item", []psElement{psArg("a", "b"), {Param: "r"}, {Param: "Fo"}})
	if b.Refusal != "" || !b.Switches["recurse"] || !b.Switches["force"] || len(b.Params["path"]) != 2 {
		t.Fatalf("Remove-Item a,b -r -Fo bound %+v", b)
	}
	b = psBind("copy-item", []psElement{psArg("src"), psArg("dst")})
	if b.Params["path"][0].Text != "src" || b.Params["destination"][0].Text != "dst" {
		t.Fatalf("Copy-Item src dst bound %+v", b)
	}
	b = psBind("remove-item", []psElement{{Param: "LP"}, psArg("x"), {Param: "Recurse", Colon: true, Values: []psValue{{Text: "False"}}}})
	if b.Params["literalpath"][0].Text != "x" || b.Switches["recurse"] {
		t.Fatalf("Remove-Item -LP x -Recurse:False bound %+v", b)
	}
	b = psBind("remove-item", []psElement{{Param: "-"}, psArg("-Recurse")})
	if b.Params["path"][0].Text != "-Recurse" || b.Switches["recurse"] {
		t.Fatalf("Remove-Item -- -Recurse bound %+v", b)
	}
	b = psBind("out-file", []psElement{{Param: "Path"}, psArg("o.txt")})
	if b.Params["filepath"][0].Text != "o.txt" {
		t.Fatalf("Out-File -Path o.txt bound %+v", b)
	}
	b = psBind("remove-item", []psElement{psArg("x"), {Param: "ea"}, psArg("SilentlyContinue")})
	if b.Refusal != "" || b.Params["erroraction"][0].Text != "SilentlyContinue" {
		t.Fatalf("Remove-Item x -ea SilentlyContinue bound %+v", b)
	}
	if b := psBind("get-widget", []psElement{{Param: "Anything"}}); b.Refusal != "" {
		t.Fatalf("an unmodelled cmdlet refused: %s", b.Refusal)
	}
}

func TestPsBindRefusals(t *testing.T) {
	cases := map[string][]psElement{
		"ambiguous":  {psArg("x"), {Param: "c"}},
		"no param":   {psArg("x"), {Param: "Bogus"}},
		"twice":      {{Param: "Recurse"}, {Param: "Recurse"}},
		"no value":   {{Param: "Path"}},
		"positional": {psArg("a"), psArg("b")},
		"splat":      {{Splat: true, Values: []psValue{{Text: "@p", Dynamic: true}}}},
	}
	for name, elems := range cases {
		if b := psBind("remove-item", elems); b.Refusal == "" {
			t.Errorf("%s: no refusal, bound %+v", name, b)
		}
	}
}

func TestPsCmdletArgs(t *testing.T) {
	args, dynamic := psCmdletArgs("remove-item", []psElement{
		psParamElem("Path", "C:/x"), psParamElem("Recurse", "True"), psArg("a", "b"),
		{Values: []psValue{{Text: "$t", Dynamic: true}}},
	})
	want := []string{"-Path", "C:/x", "-Recurse:True", "a", "b", "$t"}
	if !reflect.DeepEqual(args, want) || !reflect.DeepEqual(dynamic, []bool{false, false, false, false, false, true}) {
		t.Fatalf("psCmdletArgs = %q %v", args, dynamic)
	}
	if psParameter(args, "recurse", 1) == "" {
		t.Fatalf("psParameter does not see the switch in %q", args)
	}
}

func TestPsCmdletParamsCoverEquivalents(t *testing.T) {
	for _, cmdlet := range []string{"remove-item", "copy-item", "move-item", "rename-item", "set-content", "add-content", "clear-content", "out-file", "new-item", "get-content", "set-location", "push-location", "invoke-webrequest", "invoke-restmethod"} {
		params, ok := psCmdletParams[cmdlet]
		if !ok || len(params) == 0 {
			t.Errorf("%s has no parameter table", cmdlet)
		}
		for _, param := range params {
			if param.Name != strings.ToLower(param.Name) {
				t.Errorf("%s parameter %s is not lower case", cmdlet, param.Name)
			}
		}
	}
}
