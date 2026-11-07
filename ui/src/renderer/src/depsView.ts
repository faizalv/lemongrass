// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { DepEntry, DepOffer, DepsReport } from '../../preload/types'

// lgrassd reports features as fixed ids; an id with no copy here is shown as it is.
const FEATURES: Record<string, string> = {
  gitPane: 'the Git pane',
  conflicts: 'conflict resolution',
  rebaseForecast: 'the rebase forecast',
  biblioHistory: 'biblio history',
  workspaces: 'workspaces',
  device: 'the Device pane',
  flutterRecording: 'Flutter test recording',
  sshChannels: 'database channels over SSH',
  claudeTabs: 'Claude Code tabs',
  codexTabs: 'Codex tabs'
}

export function listText(items: string[], last: 'and' | 'or' = 'and'): string {
  if (items.length <= 1) return items[0] ?? ''
  return `${items.slice(0, -1).join(', ')} ${last} ${items[items.length - 1]}`
}

// The name a row shows: the agent's own name, such as Claude Code, or the binary's.
export function depLabel(dep: DepEntry): string {
  return dep.label || dep.name
}

// The agent rows, and whether any of them can run. Lemongrass needs at least one agent.
export function agentDeps(report: DepsReport | null | undefined): DepEntry[] {
  return report?.deps.filter((dep) => dep.agent) ?? []
}

export function anyAgentReady(report: DepsReport | null | undefined): boolean {
  return agentDeps(report).some((dep) => dep.status === 'ok')
}

// Said above the agent rows while none can run.
export function agentGroupText(agents: DepEntry[]): string {
  return `Lemongrass runs ${listText(agents.map(depLabel), 'or')} in its tabs. Install at least one.`
}

// Whether the check found the agent a New shell pad would start missing. Before the first report nothing is known,
// so nothing counts as missing.
export function agentMissing(report: DepsReport | null | undefined, command: string): boolean {
  const dep = report?.deps.find((entry) => entry.agent && entry.name === command)
  return dep?.status === 'missing'
}

// Said under an agent whose folder the login shell's PATH lacks.
export function offPathText(dep: DepEntry, dir: string): string {
  return `${depLabel(dep)} is in ${dir}, which is not on your shell's PATH. Lemongrass finds it there. If a terminal does not, add ${dir} to your shell profile.`
}

export function featuresText(ids: string[]): string {
  if (ids.length === 0) return ''
  return `Used by ${listText(ids.map((id) => FEATURES[id] ?? id))}.`
}

export function statusText(dep: DepEntry): string {
  switch (dep.status) {
    case 'ok':
      return dep.version || 'Installed'
    case 'missing':
      return 'Not installed'
    case 'belowFloor':
      return `${dep.version} is older than ${dep.floor}`
    case 'unknownVersion':
      return 'Version unknown'
  }
}

export function originText(dep: DepEntry): string {
  switch (dep.origin.kind) {
    case 'package':
      switch (dep.origin.manager) {
        case 'winget':
          return `from winget, ${dep.origin.package}`
        case 'scoop':
          return 'from Scoop'
        case 'installer':
          return 'from the Git for Windows installer'
        case 'windows':
          return 'part of Windows'
        case 'vendor':
          return 'from its official installer'
        case 'npm':
          return `from npm, ${dep.origin.package}`
        case 'brew':
          return `from Homebrew, ${dep.origin.package}`
      }
      return `from ${dep.origin.package}`
    case 'manual':
      return 'installed by hand'
    default:
      return ''
  }
}

// A required dependency that is not usable gets the primary button, and so does a missing agent while no agent can
// run; an optional one gets a ghost button, and one with nothing to offer no button at all.
export function buttonWeight(dep: DepEntry, agentNeeded = false): 'primary' | 'ghost' | null {
  if (!dep.offer) return null
  const needed = dep.required || (dep.agent && agentNeeded)
  return needed && dep.status !== 'ok' ? 'primary' : 'ghost'
}

export function actionLabel(offer: DepOffer): string {
  return offer.action === 'upgrade' ? 'Update' : 'Install'
}

// Why an offer's button is off, or null. The Linux helper and winget run from the installed app only, which
// unsupported says; an agent's vendor script runs anywhere; Homebrew does not yet. Main also refuses a route that is
// not this platform's own.
export function routeBlocked(offer: DepOffer, unsupported: string | null): string | null {
  if (offer.route === 'vendor') return null
  if (offer.route === 'helper' || offer.route === 'winget') return unsupported
  return 'Installing this from lemongrass is not supported on this system yet.'
}

// Said under an install button before it is pressed: winget accepts the package's license on the user's behalf, and
// an agent's own installer runs as the user and may edit the user's PATH or shell profile.
export function routeNote(dep: DepEntry, family: string): string | null {
  const offer = dep.offer
  if (!offer) return null
  if (offer.route === 'winget') {
    return `Installs through winget, which accepts the license of ${offer.package} for you.`
  }
  if (offer.route !== 'vendor') return null
  if (dep.name === 'claude') {
    return "Runs Anthropic's official installer for your user. Claude Code then updates itself."
  }
  if (dep.name === 'codex') {
    return family === 'windows'
      ? "Runs OpenAI's official installer for your user, which adds its folder to your PATH."
      : "Runs OpenAI's official installer for your user, which adds ~/.local/bin to your shell profile."
  }
  return `Runs the official installer of ${offer.package} for your user.`
}

export function tone(dep: DepEntry): 'ok' | 'warn' | 'muted' {
  if (dep.status === 'ok') return 'ok'
  return dep.required ? 'warn' : 'muted'
}

export function osText(os: { family: string; id: string; version: string }): string {
  const name = os.id ? os.id.charAt(0).toUpperCase() + os.id.slice(1) : 'Unknown system'
  return os.version ? `${name} ${os.version}` : name
}
