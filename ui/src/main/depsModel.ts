// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { DepEntry, DepOffer, DepOrigin, DepOutcome, DepsReport } from '../preload/types'

// Like gitMove.ts, this file has no value imports so node --test can load it straight from source.

const STATUSES = ['ok', 'missing', 'belowFloor', 'unknownVersion']
const ORIGINS = ['package', 'manual', 'none', 'unknown']
const ROUTES = ['helper', 'winget', 'brew', 'vendor']
export const KEPT_LINES = 6

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

type Manager = NonNullable<Extract<DepOrigin, { kind: 'package' }>['manager']>
const MANAGERS: string[] = [
  'winget',
  'scoop',
  'installer',
  'windows',
  'vendor',
  'npm',
  'brew'
] satisfies Manager[]

function parseOrigin(value: unknown): DepOrigin | null {
  if (!isRecord(value) || !ORIGINS.includes(text(value.kind))) return null
  if (value.kind !== 'package') return { kind: value.kind as 'manual' | 'none' | 'unknown' }
  const name = text(value.package)
  if (!name) return null
  // Windows reports a manager, and so does every agent CLI; an unknown one is dropped rather than refusing the report.
  const manager = text(value.manager)
  return MANAGERS.includes(manager)
    ? { kind: 'package', package: name, manager: manager as Manager }
    : { kind: 'package', package: name }
}

function parseOffer(value: unknown): DepOffer | null | undefined {
  if (value === null || value === undefined) return null
  if (!isRecord(value)) return undefined
  const action = text(value.action)
  const route = text(value.route)
  const name = text(value.package)
  if ((action !== 'install' && action !== 'upgrade') || !ROUTES.includes(route) || !name) {
    return undefined
  }
  return { action, package: name, route: route as DepOffer['route'] }
}

function parseEntry(value: unknown): DepEntry | null {
  if (!isRecord(value)) return null
  const name = text(value.name)
  const origin = parseOrigin(value.origin)
  const offer = parseOffer(value.offer)
  const status = text(value.status)
  if (!name || !origin || offer === undefined || !STATUSES.includes(status)) return null
  return {
    name,
    label: text(value.label),
    required: value.required === true,
    agent: value.agent === true,
    features: Array.isArray(value.features)
      ? value.features.filter((f) => typeof f === 'string')
      : [],
    floor: text(value.floor),
    path: text(value.path),
    version: text(value.version),
    origin,
    status: status as DepEntry['status'],
    offer,
    message: text(value.message)
  }
}

// The output of `lgrassd deps check --json`. A report the app cannot read in full is refused as a
// whole, since an install button built from a half-read entry could name the wrong package.
export function parseReport(stdout: string): DepsReport | null {
  let value: unknown
  try {
    value = JSON.parse(stdout)
  } catch {
    return null
  }
  if (!isRecord(value) || !isRecord(value.os) || !Array.isArray(value.deps)) return null
  const deps = value.deps.map(parseEntry)
  if (deps.some((dep) => dep === null)) return null
  return {
    os: { family: text(value.os.family), id: text(value.os.id), version: text(value.os.version) },
    deps: deps as DepEntry[]
  }
}

// The Go arch name of the helper's resource folder; the polkit policy names an action for each.
export function helperArch(arch: string): string | null {
  return arch === 'x64' ? 'amd64' : arch === 'arm64' ? 'arm64' : null
}

// Why this build cannot install anything at all, or null. The root helper ships only in the Linux rpm;
// Windows installs run winget, which the system already has.
export function installUnsupported(
  packaged: boolean,
  platform: string,
  arch: string
): string | null {
  if (!packaged) return 'This is a source build. System packages install only in the installed app.'
  if (platform === 'win32') return null
  if (platform !== 'linux' || !helperArch(arch)) {
    return 'Installing system packages from lemongrass is not supported on this system yet.'
  }
  return null
}

// The route each platform runs for system packages: the root helper on Linux, winget on Windows.
const ROUTE_OF_PLATFORM: Record<string, DepOffer['route']> = { linux: 'helper', win32: 'winget' }

// Why one offer cannot run here, or null. The vendor route needs nothing shipped in the package, so it runs in a
// source build and on macOS too, where system package installs are not built yet.
export function offerBlocked(
  offer: DepOffer | null,
  unsupported: string | null,
  platform: string
): string | null {
  if (!offer) return 'Lemongrass has nothing to install for this.'
  if (offer.route === 'vendor') {
    return VENDOR_PLATFORMS.includes(platform)
      ? null
      : 'Installing from lemongrass is not supported on this system yet.'
  }
  if (unsupported) return unsupported
  if (ROUTE_OF_PLATFORM[platform] !== offer.route)
    return 'Installing from lemongrass is not supported on this system yet.'
  return null
}

// The winget run for an offer. winget asks to accept the source's and the package's agreements on first use
// and cannot be answered without a terminal, so the accept flags stand in for that answer; the row says so
// before the button is pressed. An installer that needs admin, such as Git for Windows, still raises its own
// UAC prompt.
export function wingetArgs(offer: DepOffer): string[] {
  return [
    offer.action,
    '--id',
    offer.package,
    '--exact',
    '--source',
    'winget',
    '--silent',
    '--disable-interactivity',
    '--accept-source-agreements',
    '--accept-package-agreements'
  ]
}

// Each agent CLI's official install scripts. They are held here, keyed by dependency name, so neither the check's
// report nor the renderer can make the app fetch and run any other script. Claude's and Antigravity's shell scripts
// need bash; Codex's is written for sh.
const VENDOR_SCRIPTS: Record<string, { ps1: string; sh: string; shell: 'bash' | 'sh' }> = {
  claude: {
    ps1: 'https://claude.ai/install.ps1',
    sh: 'https://claude.ai/install.sh',
    shell: 'bash'
  },
  codex: {
    ps1: 'https://chatgpt.com/codex/install.ps1',
    sh: 'https://chatgpt.com/codex/install.sh',
    shell: 'sh'
  },
  agy: {
    ps1: 'https://antigravity.google/cli/install.ps1',
    sh: 'https://antigravity.google/cli/install.sh',
    shell: 'bash'
  }
}
const VENDOR_PLATFORMS = ['linux', 'darwin', 'win32']

// The command a user runs by hand for an agent CLI, in PowerShell on Windows and a terminal elsewhere: the one
// shown when the app's own run fails.
export function vendorCommand(name: string, platform: string): string | null {
  const script = VENDOR_SCRIPTS[name]
  if (!script || !VENDOR_PLATFORMS.includes(platform)) return null
  return platform === 'win32'
    ? `irm ${script.ps1} | iex`
    : `curl -fsSL ${script.sh} | ${script.shell}`
}

export interface VendorRun {
  file: string
  args: string[]
  env: Record<string, string>
}

// How the app runs an agent's install script. On Linux and macOS pipefail makes a failed download fail the run
// instead of piping nothing into the shell, which exits 0. On Windows the stop preference makes a failed download or
// a script that PowerShell refuses end with a non-zero code, and powershell.exe is named by its full path, so a
// powershell.exe planted earlier on PATH is never run. Codex's script asks one question, about an npm copy, unless
// CODEX_NON_INTERACTIVE is set.
export function vendorRun(name: string, platform: string, systemRoot: string): VendorRun | null {
  const command = vendorCommand(name, platform)
  if (!command) return null
  const env: Record<string, string> = name === 'codex' ? { CODEX_NON_INTERACTIVE: '1' } : {}
  if (platform === 'win32') {
    const file = systemRoot
      ? `${systemRoot}\\System32\\WindowsPowerShell\\v1.0\\powershell.exe`
      : 'powershell.exe'
    return {
      file,
      args: [
        '-NoProfile',
        '-NonInteractive',
        '-ExecutionPolicy',
        'Bypass',
        '-Command',
        `$ErrorActionPreference = 'Stop'; ${command}`
      ],
      env
    }
  }
  return { file: '/bin/bash', args: ['-c', `set -o pipefail; ${command}`], env }
}

// The sentence for a vendor script that did not exit 0, with the command to run by hand.
export function vendorExitText(
  code: number | null,
  label: string,
  lastLine: string,
  manual: string,
  platform: string
): string {
  if (code === null) return 'The install was stopped before it finished.'
  const where = platform === 'win32' ? 'in PowerShell' : 'in a terminal'
  const last = lastLine ? ` ${lastLine}` : ''
  return `The ${label} installer ended with code ${code}.${last} You can run it yourself ${where}: ${manual}`
}

// A running adb.exe is locked and cannot be replaced, and scrcpy's folder carries its own adb, so updating
// either stops the adb server first.
export function stopsAdbServer(name: string): boolean {
  return name === 'adb' || name === 'scrcpy'
}

// winget's codes are HRESULTs, which Node reports as unsigned numbers.
const WINGET_NO_UPGRADE = 0x8a15002b
const WINGET_NOT_FOUND = 0x8a150014

// The sentence for a winget run that did not exit 0.
export function wingetExitText(code: number | null, name: string, lastLine: string): string {
  if (code === null) return 'The install was stopped before it finished.'
  const unsigned = code >>> 0
  if (unsigned === WINGET_NO_UPGRADE) return `winget has no newer ${name}.`
  if (unsigned === WINGET_NOT_FOUND) return `winget could not find the package for ${name}.`
  const hex = `0x${unsigned.toString(16).toUpperCase()}`
  return lastLine
    ? `winget ended with code ${hex}. ${lastLine}`
    : `winget ended with code ${hex}. Its last output is shown above.`
}

// Adds a chunk of output to the kept lines. dnf redraws its progress bars with a carriage return, so
// that ends a line too, and only the newest lines are kept.
export function appendOutput(lines: string[], partial: string, chunk: string): [string[], string] {
  const parts = (partial + chunk).split(/\r\n|\r|\n/)
  const rest = parts.pop() ?? ''
  const added = parts.map((line) => line.trimEnd()).filter((line) => line !== '')
  return [[...lines, ...added].slice(-KEPT_LINES), rest]
}

// The sentence for how a `pkexec lgrassroot` run ended. The helper's codes are 1 (the package manager
// failed), 2 (refused, nothing ran) and 3 (another package manager holds the lock); pkexec's own are
// 126 (the prompt was dismissed) and 127 (not authorized). The helper's last stderr line starts with
// "lgrassroot: " and is shown when it has one.
export function helperExitText(code: number | null, lastError: string): string {
  const helperLine = lastError.startsWith('lgrassroot: ') ? lastError : ''
  switch (code) {
    case 126:
      return 'The password prompt was dismissed. Nothing was installed.'
    case 127:
      return 'Your account is not allowed to install packages. Nothing was installed.'
    case 2:
      return helperLine || 'Lemongrass refused this request. Nothing was installed.'
    case 3:
      return 'Another package manager is running. Try again once it finishes.'
    case 1:
      return helperLine || 'The package manager failed. Its last output is shown above.'
    case null:
      return 'The install was stopped before it finished.'
    default:
      return helperLine || `The install ended with code ${code}.`
  }
}

export type RequiredStatuses = Record<string, DepEntry['status']>

// The agent CLIs are optional one by one, but lemongrass needs at least one, so the group is recorded as one
// required item under a key no dependency name can take.
export const AGENT_GROUP = '@agents'

// The statuses the reopen rule judges: every required dependency, and the agent group as ok when any agent is.
function requiredStatuses(report: DepsReport): [string, DepEntry['status']][] {
  const entries: [string, DepEntry['status']][] = report.deps
    .filter((dep) => dep.required)
    .map((dep) => [dep.name, dep.status])
  const agents = report.deps.filter((dep) => dep.agent)
  if (agents.length > 0) {
    entries.push([AGENT_GROUP, agents.some((dep) => dep.status === 'ok') ? 'ok' : 'missing'])
  }
  return entries
}

// Whether a required dependency that was ok when last seen is no longer ok, which reopens the startup layer.
export function requiredRegressed(seen: RequiredStatuses, report: DepsReport): boolean {
  return requiredStatuses(report).some(([name, status]) => seen[name] === 'ok' && status !== 'ok')
}

// The statuses to keep after a check. A required dependency now ok is recorded at once; one that is
// not ok keeps its earlier record until the layer completes, so a layer closed without Continue
// still reopens on the next startup.
export function nextSeen(
  seen: RequiredStatuses,
  report: DepsReport,
  completed: boolean
): RequiredStatuses {
  const next: RequiredStatuses = {}
  for (const [name, status] of requiredStatuses(report)) {
    next[name] = completed || status === 'ok' ? status : (seen[name] ?? status)
  }
  return next
}

// The folder of each agent CLI that is not on the login shell's PATH, on Linux and macOS: a terminal outside
// lemongrass may not find it, which the row says, while lemongrass's own tabs find it (the app adds ~/.local/bin to
// its own PATH and falls back to the checked path). An unread login PATH reports nothing. On Windows both vendor
// folders are on the user's PATH (lemongrass adds ~/.local/bin, Codex's script adds its own), so nothing is reported.
export function agentsOffPath(
  report: DepsReport,
  pathVar: string | null,
  platform: string
): Record<string, string> {
  if (platform === 'win32' || pathVar === null) return {}
  const dirs = new Set(pathVar.split(':').map((dir) => dir.replace(/\/+$/, '')))
  const off: Record<string, string> = {}
  for (const dep of report.deps) {
    if (!dep.agent || !dep.path) continue
    const dir = dep.path.slice(0, dep.path.lastIndexOf('/'))
    if (dir && !dirs.has(dir)) off[dep.name] = dir
  }
  return off
}

// The sentence for a helper run that exited 0. dnf also exits 0 with "Nothing to do" when the
// repositories have no newer package, so the check that follows decides whether it fixed anything.
export function successText(
  action: DepOffer['action'],
  after: DepEntry | undefined,
  route: DepOffer['route'] = 'helper'
): DepOutcome {
  if (after && after.status !== 'ok') {
    return {
      ok: false,
      text:
        action === 'upgrade'
          ? route === 'winget'
            ? `winget has no newer ${after.name}.`
            : `Your repositories have no newer ${after.name}.`
          : `${after.label || after.name} is still not usable after the install.`
    }
  }
  return { ok: true, text: action === 'upgrade' ? 'Updated.' : 'Installed.' }
}
