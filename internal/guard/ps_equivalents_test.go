// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"reflect"
	"testing"
)

func psEquivalentsOf(cmdlet string, fromPipeline bool, elems ...psElement) ([]shellCall, []shellRedirect) {
	return psEquivalents(psBind(cmdlet, elems), fromPipeline)
}

func TestPsEquivalentsRemove(t *testing.T) {
	calls, _ := psEquivalentsOf("remove-item", false, psArg("a", "b"), psElement{Param: "Recurse"}, psElement{Param: "Force"})
	want := shellCall{Name: "rm", Args: []string{"-r", "-f", "--", "a", "b"}, ArgDynamic: []bool{false, false, false, false, false}}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
		t.Fatalf("Remove-Item a,b -Recurse -Force = %+v", calls)
	}
	calls, _ = psEquivalentsOf("remove-item", false, psArg("f.txt"))
	if !reflect.DeepEqual(calls[0].Args, []string{"--", "f.txt"}) {
		t.Fatalf("Remove-Item f.txt = %+v", calls)
	}
	calls, _ = psEquivalentsOf("remove-item", false, psElement{Param: "LiteralPath"}, psElement{Values: []psValue{{Text: "$t", Dynamic: true}}}, psElement{Param: "Recurse"})
	if !calls[0].ArgDynamic[2] {
		t.Fatalf("a dynamic target is not marked: %+v", calls)
	}
	calls, _ = psEquivalentsOf("remove-item", true, psElement{Param: "Recurse"})
	if len(calls) != 1 || !calls[0].Indirect {
		t.Fatalf("a pipeline-fed Remove-Item = %+v", calls)
	}
	calls, _ = psEquivalentsOf("remove-item", false, psArg(`HKCU:\Software\x`), psElement{Param: "Recurse"})
	if len(calls) != 1 || calls[0].Name != "reg" || calls[0].Args[0] != "delete" {
		t.Fatalf("Remove-Item on the registry = %+v", calls)
	}
	calls, _ = psEquivalentsOf("remove-item", false, psArg(`Proj:\src`))
	if calls[0].Name != "rm" || !calls[0].ArgDynamic[1] {
		t.Fatalf("Remove-Item on an unknown drive = %+v", calls)
	}
	if calls, _ := psEquivalentsOf("remove-item", false, psArg("env:X")); len(calls) != 0 {
		t.Fatalf("Remove-Item env:X = %+v", calls)
	}
}

func TestPsEquivalentsFileCore(t *testing.T) {
	calls, _ := psEquivalentsOf("copy-item", false, psArg("a"), psArg("b"), psElement{Param: "Recurse"})
	if !reflect.DeepEqual(calls[0].Args, []string{"-r", "--", "a", "b"}) || calls[0].Name != "cp" {
		t.Fatalf("Copy-Item = %+v", calls)
	}
	calls, _ = psEquivalentsOf("move-item", false, psArg("a"))
	if calls[0].Name != "mv" || !reflect.DeepEqual(calls[0].Args, []string{"--", "a", "."}) {
		t.Fatalf("Move-Item without -Destination = %+v", calls)
	}
	calls, _ = psEquivalentsOf("rename-item", false, psArg(`C:\p\old.txt`), psArg("new.txt"))
	if !reflect.DeepEqual(calls[0].Args, []string{"--", `C:\p\old.txt`, `C:\p\new.txt`}) {
		t.Fatalf("Rename-Item = %+v", calls)
	}
	_, redirects := psEquivalentsOf("set-content", false, psArg("~/.bashrc"), psArg("x"))
	if !reflect.DeepEqual(redirects, []shellRedirect{{Path: "~/.bashrc", Write: true}}) {
		t.Fatalf("Set-Content = %+v", redirects)
	}
	_, redirects = psEquivalentsOf("out-file", false, psElement{Param: "FilePath"}, psArg("o.txt"))
	if len(redirects) != 1 || redirects[0].Path != "o.txt" {
		t.Fatalf("Out-File = %+v", redirects)
	}
	calls, _ = psEquivalentsOf("clear-content", false, psArg("log.txt"))
	if calls[0].Name != "truncate" {
		t.Fatalf("Clear-Content = %+v", calls)
	}
	calls, _ = psEquivalentsOf("get-content", false, psArg(`~\.ssh\id_rsa`))
	if calls[0].Name != "cat" || calls[0].Args[1] != `~\.ssh\id_rsa` {
		t.Fatalf("Get-Content = %+v", calls)
	}
}

func TestPsEquivalentsNewItem(t *testing.T) {
	calls, _ := psEquivalentsOf("new-item", false, psArg("d"), psElement{Param: "ItemType"}, psArg("Directory"))
	if calls[0].Name != "mkdir" {
		t.Fatalf("New-Item -ItemType Directory = %+v", calls)
	}
	calls, _ = psEquivalentsOf("new-item", false, psArg("link"), psElement{Param: "ItemType"}, psArg("SymbolicLink"), psElement{Param: "Target"}, psArg("~/.ssh"))
	if calls[0].Name != "ln" || !reflect.DeepEqual(calls[0].Args, []string{"-s", "--", "~/.ssh", "link"}) {
		t.Fatalf("New-Item symlink = %+v", calls)
	}
	_, redirects := psEquivalentsOf("new-item", false, psArg("dir"), psElement{Param: "Name"}, psArg("f.txt"))
	if len(redirects) != 1 || redirects[0].Path != "dir/f.txt" {
		t.Fatalf("New-Item -Name = %+v", redirects)
	}
	calls, _ = psEquivalentsOf("new-item", false, psArg(`HKCU:\Software\x`))
	if calls[0].Name != "reg" || calls[0].Args[0] != "add" {
		t.Fatalf("New-Item on the registry = %+v", calls)
	}
}

func TestPsEquivalentsLocationAndWeb(t *testing.T) {
	calls, _ := psEquivalentsOf("set-location", false, psArg("src"))
	if dir, ok := directoryTarget(calls[0]); !ok || dir != "src" || calls[0].Name != "cd" {
		t.Fatalf("Set-Location src = %+v", calls)
	}
	for _, target := range []psElement{psArg(`HKCU:\`), psArg(`Proj:\`), {Values: []psValue{{Text: "(Resolve-Path x).Path", Dynamic: true}}}} {
		calls, _ = psEquivalentsOf("set-location", false, target)
		if _, ok := directoryTarget(calls[0]); ok {
			t.Fatalf("Set-Location %v is followed: %+v", target, calls)
		}
	}
	calls, _ = psEquivalentsOf("invoke-webrequest", false, psArg("https://x/a.sh"), psElement{Param: "OutFile"}, psArg("a.sh"))
	if calls[0].Name != "curl" || !reflect.DeepEqual(calls[0].Args, []string{"-o", "a.sh", "https://x/a.sh"}) {
		t.Fatalf("Invoke-WebRequest -OutFile = %+v", calls)
	}
}
