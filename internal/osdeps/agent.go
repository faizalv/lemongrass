// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package osdeps

import (
	"fmt"
	"path"
	"strings"
)

// evaluateAgent offers an agent CLI's install through its vendor's script when it is missing, and nothing once it is
// there, since every agent updates itself. On Linux and macOS the script is fetched with curl.
func evaluateAgent(dep Dep, info OSInfo, binPath, version string, origin Origin) DepStatus {
	s := newStatus(dep, binPath, version, origin)
	if s.Status != StatusMissing {
		return s
	}
	if info.Family != FamilyWindows && !info.Curl {
		s.Message = fmt.Sprintf("Lemongrass installs %s with curl, which this system does not have. Install curl, then check again.", dep.Label)
		return s
	}
	s.Offer = &Offer{Action: ActionInstall, Package: dep.Label, Route: RouteVendor}
	return s
}

// The package each agent CLI has under npm and Homebrew, named in its origin.
var agentPackages = map[string]struct{ npm, brew string }{
	"claude": {npm: "@anthropic-ai/claude-code", brew: "claude-code"},
	"codex":  {npm: "@openai/codex", brew: "codex"},
}

// agentOrigin names who installed an agent CLI from the folder its real file is in; a vendor script's copy carries
// the agent's own name as its package. ok is false when no known folder holds it, which leaves the OS's own origin
// check to decide.
func agentOrigin(sys System, dep Dep, binPath string) (Origin, bool) {
	real, err := sys.EvalSymlinks(binPath)
	if err != nil {
		real = binPath
	}
	npm := Origin{Kind: OriginPackage, Package: agentPackages[dep.Name].npm, Manager: ManagerNpm}
	vendor := Origin{Kind: OriginPackage, Package: dep.Label, Manager: ManagerVendor}
	if sys.GOOS == "windows" {
		return windowsAgentOrigin(sys, binPath, real, npm, vendor)
	}
	home := sys.Home
	under := func(root string) bool {
		return strings.HasPrefix(real, root+"/") || strings.HasPrefix(binPath, root+"/")
	}
	switch {
	// npm's global prefix can itself sit under ~/.local, so it is told apart first.
	case strings.Contains(real, "/node_modules/"):
		return npm, true
	case under("/opt/homebrew"), under("/usr/local/Caskroom"), under("/usr/local/Cellar"), under("/home/linuxbrew/.linuxbrew"):
		return Origin{Kind: OriginPackage, Package: agentPackages[dep.Name].brew, Manager: ManagerBrew}, true
	case home != "" && (under(path.Join(home, ".local")) || under(path.Join(home, ".codex", "packages"))):
		return vendor, true
	}
	return Origin{}, false
}

func windowsAgentOrigin(sys System, binPath, real string, npm, vendor Origin) (Origin, bool) {
	under := func(root string) bool {
		if _, ok := winUnder(real, root); ok {
			return true
		}
		_, ok := winUnder(binPath, root)
		return ok
	}
	if appData := sys.Getenv("APPDATA"); appData != "" && under(winJoin(appData, "npm")) {
		return npm, true
	}
	local := sys.Getenv("LOCALAPPDATA")
	if (sys.Home != "" && under(winJoin(sys.Home, ".local"))) ||
		(local != "" && under(winJoin(local, "Programs", "OpenAI", "Codex"))) {
		return vendor, true
	}
	return Origin{}, false
}
