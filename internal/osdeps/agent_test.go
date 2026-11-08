// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package osdeps

import (
	"strings"
	"testing"
)

const (
	claudeVersionOut = "2.1.317 (Claude Code)\n"
	codexVersionOut  = "codex-cli 0.166.0\n"
	agyVersionOut    = "1.5.3\n"
)

func TestParseAgentVersions(t *testing.T) {
	cases := []struct{ name, out, want string }{
		{"claude", claudeVersionOut, "2.1.317"},
		{"codex", codexVersionOut, "0.166.0"},
		{"codex", "codex-cli 0.158.0-alpha.2\n", "0.158.0"},
		{"codex", "something else\n", ""},
		{"agy", agyVersionOut, "1.5.3"},
	}
	for _, c := range cases {
		if got := ParseVersion(c.name, c.out); got != c.want {
			t.Errorf("ParseVersion(%s, %q) = %q, want %q", c.name, c.out, got, c.want)
		}
	}
}

func TestCheckAgentsMissingOfferTheVendorScript(t *testing.T) {
	m := newFedora()
	m.onPath["curl"] = "/usr/bin/curl"
	got := check(t, m)
	for name, label := range map[string]string{"claude": "Claude Code", "codex": "Codex", "agy": "Antigravity"} {
		d := got[name]
		if !d.Agent || d.Required || d.Label != label || d.Status != StatusMissing || d.Message != "" ||
			d.Offer == nil || *d.Offer != (Offer{ActionInstall, label, RouteVendor}) {
			t.Errorf("%s = %+v", name, d)
		}
	}
}

func TestCheckAgentsWithoutCurlAreNotOffered(t *testing.T) {
	m := newFedora()
	d := check(t, m)["claude"]
	if d.Offer != nil || !strings.Contains(d.Message, "curl") {
		t.Errorf("claude = %+v", d)
	}
}

func TestCheckAgentsOnWindowsNeedNoWinget(t *testing.T) {
	m := newWindows()
	delete(m.onPath, "winget")
	for _, name := range []string{"claude", "codex", "agy"} {
		if d := check(t, m)[name]; d.Offer == nil || d.Offer.Route != RouteVendor {
			t.Errorf("%s = %+v", name, d)
		}
	}
}

// A CLI in the vendor's folder is found though the app's PATH misses it, and an installed agent is never offered an
// upgrade, since it updates itself.
func TestCheckAgentFoundInVendorFolder(t *testing.T) {
	m := newFedora()
	m.onPath["curl"] = "/usr/bin/curl"
	bin := "/home/u/.local/bin/claude"
	m.files[bin] = FileStat{Size: 100, ModTime: 1, Exec: true}
	m.links[bin] = "/home/u/.local/share/claude/versions/2.1.317"
	m.commands[bin+" --version"] = fakeOut{out: claudeVersionOut}
	d := check(t, m)["claude"]
	if d.Status != StatusOK || d.Path != bin || d.Version != "2.1.317" || d.Offer != nil || d.Message != "" ||
		d.Origin != (Origin{Kind: OriginPackage, Package: "Claude Code", Manager: ManagerVendor}) {
		t.Errorf("claude = %+v", d)
	}
	if m.callCount("rpm ") != 0 {
		t.Errorf("rpm ran for an agent in a vendor folder")
	}
}

func TestCheckAgentOrigins(t *testing.T) {
	cases := []struct {
		name, goos, path, link, versionOut string
		want                               Origin
	}{
		{"codex", "linux", "/usr/local/bin/codex", "/usr/local/lib/node_modules/@openai/codex/bin/codex.js",
			codexVersionOut, Origin{Kind: OriginPackage, Package: "@openai/codex", Manager: ManagerNpm}},
		{"codex", "linux", "/home/u/.local/bin/codex", "/home/u/.codex/packages/standalone/codex",
			codexVersionOut, Origin{Kind: OriginPackage, Package: "Codex", Manager: ManagerVendor}},
		{"claude", "darwin", "/opt/homebrew/bin/claude", "/opt/homebrew/Caskroom/claude-code/2.1.317/claude",
			claudeVersionOut, Origin{Kind: OriginPackage, Package: "claude-code", Manager: ManagerBrew}},
		{"claude", "linux", "/usr/bin/claude", "", claudeVersionOut, Origin{Kind: OriginPackage, Package: "claude-code"}},
		{"agy", "linux", "/home/u/.local/bin/agy", "", agyVersionOut,
			Origin{Kind: OriginPackage, Package: "Antigravity", Manager: ManagerVendor}},
	}
	for _, c := range cases {
		m := newFedora()
		m.goos = c.goos
		if c.goos == "darwin" {
			m.osRelease = ""
		}
		m.install(c.name, c.path, c.versionOut, "claude-code")
		if c.link != "" {
			m.links[c.path] = c.link
		}
		if got := check(t, m)[c.name]; got.Origin != c.want || got.Status != StatusOK || got.Offer != nil {
			t.Errorf("%s at %s = %+v", c.name, c.path, got)
		}
	}
}

func TestCheckAgentsOnWindows(t *testing.T) {
	m := newWindows()
	m.env["APPDATA"] = `C:\Users\a\AppData\Roaming`
	claude := `C:\Users\a\.local\bin\claude.exe`
	m.files[claude] = FileStat{Size: 100, ModTime: 1, Exec: true}
	m.commands[claude+" --version"] = fakeOut{out: claudeVersionOut}
	codex := `C:\Users\a\AppData\Roaming\npm\codex.cmd`
	m.installWin("codex", codex, codexVersionOut)
	got := check(t, m)
	if d := got["claude"]; d.Path != claude || d.Version != "2.1.317" || d.Offer != nil ||
		d.Origin != (Origin{Kind: OriginPackage, Package: "Claude Code", Manager: ManagerVendor}) {
		t.Errorf("claude = %+v", d)
	}
	if d := got["codex"]; d.Path != codex || d.Offer != nil || d.Origin != (Origin{Kind: OriginPackage, Package: "@openai/codex", Manager: ManagerNpm}) {
		t.Errorf("codex = %+v", d)
	}

	m = newWindows()
	vendor := winLocal + `\Programs\OpenAI\Codex\bin\codex.exe`
	m.files[vendor] = FileStat{Size: 100, ModTime: 1, Exec: true}
	m.commands[vendor+" --version"] = fakeOut{out: codexVersionOut}
	if d := check(t, m)["codex"]; d.Path != vendor || d.Origin != (Origin{Kind: OriginPackage, Package: "Codex", Manager: ManagerVendor}) {
		t.Errorf("codex from its script = %+v", d)
	}

	m = newWindows()
	agy := winLocal + `\agy\bin\agy.exe`
	m.files[agy] = FileStat{Size: 100, ModTime: 1, Exec: true}
	m.commands[agy+" --version"] = fakeOut{out: agyVersionOut}
	if d := check(t, m)["agy"]; d.Path != agy || d.Version != "1.5.3" || d.Offer != nil ||
		d.Origin != (Origin{Kind: OriginPackage, Package: "Antigravity", Manager: ManagerVendor}) {
		t.Errorf("agy from its script = %+v", d)
	}
}

// Antigravity has no npm or Homebrew package, so a copy under node_modules or Homebrew is left to the OS check
// rather than given a package origin with no package.
func TestAgyOriginNeverNamesAMissingPackage(t *testing.T) {
	m := newFedora()
	m.goos = "darwin"
	m.osRelease = ""
	m.install("agy", "/opt/homebrew/bin/agy", agyVersionOut, "agy")
	if got := check(t, m)["agy"]; got.Origin.Manager == ManagerBrew || got.Origin.Manager == ManagerNpm {
		t.Errorf("agy under Homebrew = %+v", got)
	}
}
