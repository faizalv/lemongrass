// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPsProviderPath(t *testing.T) {
	cases := []struct {
		path   string
		kind   psProvider
		fsPath string
	}{
		{`C:\Users\x`, psProviderFileSystem, `C:\Users\x`},
		{`c:x`, psProviderFileSystem, `c:x`},
		{`~\.ssh\id_rsa`, psProviderFileSystem, `~\.ssh\id_rsa`},
		{`HKCU:\Software\x`, psProviderRegistry, ""},
		{`hklm:Software`, psProviderRegistry, ""},
		{`Registry::HKEY_CURRENT_USER\x`, psProviderRegistry, ""},
		{`Microsoft.PowerShell.Core\Registry::HKLM\x`, psProviderRegistry, ""},
		{`FileSystem::C:\x`, psProviderFileSystem, `C:\x`},
		{`Microsoft.PowerShell.Core\FileSystem::\\srv\share`, psProviderFileSystem, `\\srv\share`},
		{`Cert:\CurrentUser\My`, psProviderCertificate, ""},
		{`env:PATH`, psProviderEnvironment, ""},
		{`variable:x`, psProviderVariable, ""},
		{`alias:ls`, psProviderAlias, ""},
		{`Function:prompt`, psProviderFunction, ""},
		{`WSMan:\localhost`, psProviderWSMan, ""},
		{`Proj:\src`, psProviderUnknown, ""},
		{`Bogus::x`, psProviderUnknown, ""},
		{`Temp:`, psProviderTemp, os.TempDir()},
		{`Temp:\a\b`, psProviderTemp, filepath.Join(os.TempDir(), `a\b`)},
	}
	for _, c := range cases {
		kind, fsPath := psProviderPath(c.path)
		if kind != c.kind || fsPath != c.fsPath {
			t.Errorf("psProviderPath(%q) = %d %q, want %d %q", c.path, kind, fsPath, c.kind, c.fsPath)
		}
	}
}

func TestPsProviderForm(t *testing.T) {
	registry := psProviderForm(true, psProviderRegistry)
	if _, ok := registry(shellCall{Name: "remove-item", Args: []string{"-Path", `HKCU:\Software\x`}}, matchEnv{}); !ok {
		t.Error("Remove-Item on HKCU: did not match")
	}
	if _, ok := registry(shellCall{Name: "get-item", Args: []string{`HKCU:\Software\x`}}, matchEnv{}); ok {
		t.Error("Get-Item on HKCU: matched a write form")
	}
	if _, ok := registry(shellCall{Name: "set-item", Args: []string{"$p"}, ArgDynamic: []bool{true}}, matchEnv{}); ok {
		t.Error("a dynamic argument matched")
	}
	cert := psProviderForm(false, psProviderCertificate)
	if _, ok := cert(shellCall{Name: "get-childitem", Args: []string{`Cert:\CurrentUser\My`, "-Recurse"}}, matchEnv{}); !ok {
		t.Error("Get-ChildItem Cert: did not match")
	}
}
