// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

// Set by the Electron app on a Shell tab only, never on an agent tab, and inherited by everything the tab runs. An
// agent started inside a Shell tab has no LGRASS_TAB_ID, so lemongrass cannot route, restore or orient it. The tab's
// shims stop the plain command; this guard catches every other way in (a full path, `command claude`, npx, a script).
const shellTabEnv = "LGRASS_SHELL_TAB"

// The hidden `lgrassd hook` argument that runs the detached ender, see startShellTabEnd.
const shellTabEndArg = "end-shell-job"

const shellTabMessage = "lemongrass: Claude Code and Codex do not run inside a Shell tab, because lemongrass cannot track them there. Open an agent tab from New shell instead."

func inShellTab(tabID string, marker string) bool {
	return tabID == "" && marker != ""
}

// Given on every event, also while the ender is ending the job, so the agent cannot act in between: every prompt is blocked and every tool call denied, and the
// other events return nothing. It is decided before project resolution, so the session never reaches the store.
func shellTabFallback(event string) hookResult {
	switch event {
	case "UserPromptSubmit":
		return hookResult{EventName: event, BlockReason: shellTabMessage}
	case "PreToolUse":
		return newHookResult(event, "deny", []string{shellTabMessage})
	}
	return hookResult{}
}
