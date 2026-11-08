// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"reflect"
	"testing"
)

func TestPsNativeArgv(t *testing.T) {
	readings, refusal := psNativeArgv("git", []psElement{psArg("commit"), {Param: "m"}, {Values: []psValue{{Text: "a b", Quoted: true}}}, {Param: "-"}, psArg("x,y")})
	want := []psNativeReading{{Args: []string{"commit", "-m", "a b", "--", "x,y"}, ArgDynamic: []bool{false, false, false, false, false}}}
	if refusal != "" || !reflect.DeepEqual(readings, want) {
		t.Fatalf("git argv = %+v %q", readings, refusal)
	}
	readings, _ = psNativeArgv("tool", []psElement{psParamElem("out", "f.txt"), psArg("a", "b")})
	if len(readings) != 2 || !reflect.DeepEqual(readings[0].Args, []string{"-out:f.txt", "a,b"}) || !reflect.DeepEqual(readings[1].Args, []string{"-out:f.txt", "a", "b"}) {
		t.Fatalf("colon and array argv = %+v", readings)
	}
	readings, _ = psNativeArgv("rm", []psElement{psArg("-rf"), psArg(`~\x`)})
	if len(readings) != 2 || readings[0].Args[1] != `~\x` || readings[1].Args[1] != `./~\x` {
		t.Fatalf("tilde readings = %+v", readings)
	}
	readings, _ = psNativeArgv("rm", []psElement{{Values: []psValue{{Text: "~", Quoted: true}}}})
	if len(readings) != 1 {
		t.Fatalf("a quoted ~ gave %d readings", len(readings))
	}
	readings, refusal = psNativeArgv("cmd", []psElement{{Values: []psValue{{Text: "", Dynamic: true}}}})
	if refusal != "" || !readings[0].ArgDynamic[0] {
		t.Fatalf("a dynamic argument = %+v %q", readings, refusal)
	}
}

func TestPsNativeArgvRefusals(t *testing.T) {
	for name, elem := range map[string]psElement{
		"empty":     {Values: []psValue{{Text: "", Quoted: true}}},
		"quote":     {Values: []psValue{{Text: `say "hi"`, Quoted: true}}},
		"curly":     {Values: []psValue{{Text: "“x”", Quoted: true}}},
		"backslash": {Values: []psValue{{Text: `C:\a b\`, Quoted: true}}},
		"splat":     {Splat: true, Values: []psValue{{Text: "@a", Dynamic: true}}},
	} {
		if _, refusal := psNativeArgv("prog", []psElement{elem}); refusal == "" {
			t.Errorf("%s: no refusal", name)
		}
	}
	if _, refusal := psNativeArgv("prog", []psElement{psArg(`C:\dir\`)}); refusal != "" {
		t.Errorf("a trailing backslash without a space refused: %s", refusal)
	}
}
