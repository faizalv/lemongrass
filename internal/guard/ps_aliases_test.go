// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"reflect"
	"testing"
)

func TestPsResolveName(t *testing.T) {
	cases := []struct {
		name string
		want []psMeaning
	}{
		{"Remove-Item", []psMeaning{{Editions: psAllEditions, Cmdlet: "remove-item"}}},
		{"REMOVE-item", []psMeaning{{Editions: psAllEditions, Cmdlet: "remove-item"}}},
		{"ri", []psMeaning{{Editions: psAllEditions, Cmdlet: "remove-item"}}},
		{"rm", []psMeaning{{Editions: psWin51 | psWin7, Cmdlet: "remove-item"}, {Editions: psUnix7, Native: "rm"}}},
		{"curl", []psMeaning{{Editions: psWin51, Cmdlet: "invoke-webrequest"}, {Editions: psWin7 | psUnix7, Native: "curl"}}},
		{"sc", []psMeaning{{Editions: psWin51, Cmdlet: "set-content"}, {Editions: psWin7 | psUnix7, Native: "sc"}}},
		{"kill", []psMeaning{{Editions: psWin51 | psWin7, Cmdlet: "stop-process"}, {Editions: psUnix7, Native: "kill"}}},
		{"cd", []psMeaning{{Editions: psAllEditions, Cmdlet: "set-location"}}},
		{"gerr", []psMeaning{{Editions: psWin51, Native: "gerr"}, {Editions: psWin7 | psUnix7, Cmdlet: "get-error"}}},
		{"git", []psMeaning{{Editions: psAllEditions, Native: "git"}}},
		{"docker-compose", []psMeaning{{Editions: psAllEditions, Native: "docker-compose"}}},
		{"git.exe", []psMeaning{{Editions: psAllEditions, Native: "git.exe"}}},
		{`C:\tools\rm.exe`, []psMeaning{{Editions: psAllEditions, Native: `C:\tools\rm.exe`}}},
		{"./run.ps1", []psMeaning{{Editions: psAllEditions, Native: "./run.ps1"}}},
		{"Get-Widget", []psMeaning{{Editions: psAllEditions, Cmdlet: "get-widget"}, {Editions: psAllEditions, Native: "Get-Widget"}}},
		{`Microsoft.PowerShell.Management\Remove-Item`, []psMeaning{{Editions: psAllEditions, Cmdlet: "remove-item"}, {Editions: psAllEditions, Native: `Microsoft.PowerShell.Management\Remove-Item`}}},
		{"cd..", []psMeaning{{Editions: psAllEditions, Cmdlet: "set-location", Fixed: []psElement{psFixedValue("..")}}}},
		{"C:", []psMeaning{{Editions: psWin51 | psWin7, Cmdlet: "set-location", Fixed: []psElement{psFixedValue("C:")}}, {Editions: psUnix7, Native: "C:"}}},
		{"exec", []psMeaning{{Editions: psWin51 | psWin7, Native: "exec"}, {Editions: psUnix7, Cmdlet: "switch-process"}}},
	}
	for _, c := range cases {
		if got := psResolveName(c.name); !reflect.DeepEqual(got, c.want) {
			t.Errorf("psResolveName(%q) = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestPsResolveNameMkdir(t *testing.T) {
	mkdir := []psElement{{Param: "ItemType", Values: []psValue{{Text: "Directory", Quoted: true}}}}
	want := []psMeaning{{Editions: psWin51 | psWin7, Cmdlet: "new-item", Fixed: mkdir}, {Editions: psUnix7, Native: "mkdir"}}
	for _, name := range []string{"mkdir", "md", "MD"} {
		got := psResolveName(name)
		if len(got) == 2 && got[1].Native != "" {
			got[1].Native = "mkdir"
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("psResolveName(%q) = %+v, want %+v", name, got, want)
		}
	}
}

func TestPsAliasTableShadows(t *testing.T) {
	// The names recon found colliding with what the POSIX rules key on.
	for name, editions := range map[string]psEdition{
		"curl": psWin51, "wget": psWin51, "sc": psWin51, "kill": psWin51 | psWin7, "mount": psWin51 | psWin7,
		"diff": psWin51 | psWin7, "ps": psWin51 | psWin7, "sort": psWin51 | psWin7, "tee": psWin51 | psWin7,
		"start": psWin51 | psWin7, "echo": psAllEditions, "r": psAllEditions, "iex": psAllEditions,
	} {
		if got := psAliasTable[name].Editions; got != editions {
			t.Errorf("alias %s editions = %b, want %b", name, got, editions)
		}
	}
}
