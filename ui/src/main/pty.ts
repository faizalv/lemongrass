// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { ipcMain, WebContents } from 'electron'
import { homedir } from 'os'
import { join } from 'path'
import * as pty from 'node-pty'
import { randomUUID } from 'crypto'
import { registerTab } from './tabSessions'
import { ACCOUNT_NAME } from './accountNames'
import { isTrustedProject } from './projects'
import { writeTrust } from './projectTrust'
import { orientationLaunch, type OrientationRole } from './orientation'
import { startupTrimArgs, startupTrimEnv } from './startupTrim'
import { classifyInput } from './humanInput'
import { destroyInstance } from './web'
import { resolveShellBinary } from './shellBinary'
import { agentPath } from './agentPaths'
import { SHELL_COMMAND, shellTabLaunch } from './shellTab'
import { agyLaunch } from './agyLaunch'
import { windowsLaunch, windowsShell, withPath, type Env, type Launch } from './winExec'
import { freshWindowsPath } from './windowsPath'

// Every shell spawns an agent binary directly, never a login shell. Codex
// receives repeated Ctrl+C during a graceful tab or app shutdown, so it can
// run its SessionEnd hook before the PTY is force-killed. On Windows Claude
// does too, because killing a ConPTY terminates every process on its console
// at once, while on Unix the hangup lets Claude end its session itself.
// Antigravity has no SessionEnd and writes its steps to disk as they happen, so
// it is killed at once like any other shell.

interface SpawnOptions {
  /** The agent binary to spawn, e.g. "claude". Never a shell like bash/zsh. */
  command: string
  /** Extra argv passed to the agent binary, e.g. ["--append-system-prompt", "..."]. */
  args?: string[]
  /** Working directory: the project the shell is scoped to. */
  cwd?: string
  /** Workspace tab id, exposed to the agent process so its session hooks can tie a session to this tab. */
  tabId?: string
  /** Claude account name; the agent then runs with that account's config directory. */
  account?: string
  /** Which orientation a Claude tab starts with. */
  orientation?: OrientationRole
  cols?: number
  rows?: number
}

interface ManagedShell {
  process: pty.IPty
  command: string
  tabId?: string
  cwd: string
  lastInputAt: number
}

// A typed nudge waits this long after the human's last keystroke so it does not land mid-word. A plain Enter ends the wait, and anything still held reaches the model with the human's next prompt through the UserPromptSubmit hook.
const HUMAN_QUIET_MS = 5_000

const shells = new Map<string, ManagedShell>()
const stopping = new Map<string, Promise<void>>()
let shuttingDown = false
// Ctrl+C clears a draft, interrupts a running turn, and quits on a repeat press once the composer is idle.
const INTERRUPT = '\x03'
const INTERRUPT_INTERVAL_MS = 400
const INTERRUPT_EXIT_WAIT_MS = 3_000

// proc.onData/onExit fire on the child process's own timing, not the
// window's. A shell can still be emitting output while the window is
// mid-teardown, when getSender() may return a wrapper whose underlying
// native object is already gone. .send() on that throws "Object has been
// destroyed", so every send here goes through this guard.
function send(sender: WebContents | undefined, channel: string, payload: unknown): void {
  if (sender && !sender.isDestroyed()) sender.send(channel, payload)
}

function accountConfigDir(opts: SpawnOptions): string | undefined {
  if (opts.command !== 'claude' || !opts.account || !ACCOUNT_NAME.test(opts.account))
    return undefined
  return join(homedir(), `.claude-${opts.account}`)
}

function gracefulClose(command: string): boolean {
  return command === 'codex' || (command === 'claude' && process.platform === 'win32')
}

// The program and argv for a tab. On Windows the PATH is reread from the registry for every tab, and a name is found
// through PATHEXT, since ConPTY only appends .exe and an npm install is a .cmd shim.
async function launchFor(
  command: string,
  args: string[],
  env: Env
): Promise<{ launch: Launch; env: Env }> {
  if (process.platform !== 'win32')
    return { launch: { file: resolveShellBinary(command, env, agentPath(command)), args }, env }
  const winEnv = withPath(env, await freshWindowsPath())
  const launch =
    command === 'zsh'
      ? { file: windowsShell(winEnv), args }
      : windowsLaunch(command, args, winEnv, undefined, agentPath(command))
  return { launch, env: winEnv }
}

// Antigravity is trusted per session through agyLaunch's --settings, so it has no trust file to write.
function trustTarget(opts: SpawnOptions): string | undefined {
  if (opts.command === 'codex') return 'codex'
  if (opts.command !== 'claude') return undefined
  return opts.account && ACCOUNT_NAME.test(opts.account) ? opts.account : 'primary'
}

// Vendors whose hook scopes project policy and the deletion exemption to LGRASS_PROJECT_DIR.
const PROJECT_DIR_COMMANDS = new Set(['claude', 'codex', 'agy'])

export function registerPtyHandlers(
  getSender: () => WebContents | undefined,
  lgrassdPath: string | null
): void {
  ipcMain.handle('pty:spawn', async (_event, opts: SpawnOptions) => {
    const id = randomUUID()
    const orientation = orientationLaunch(
      opts.command,
      opts.orientation ?? 'tab',
      homedir(),
      opts.cwd,
      opts.args ?? []
    )
    // First, ahead of the bypass flag and any prompt, because agy takes the argument after -p as its prompt.
    const agy =
      opts.command === 'agy'
        ? agyLaunch(opts.args ?? [], opts.cwd, lgrassdPath, homedir())
        : { args: [], env: {} }
    const args = [
      ...agy.args,
      ...orientation.args,
      ...startupTrimArgs(opts.command, homedir()),
      ...(opts.args ?? [])
    ]
    const configDir = accountConfigDir(opts)
    if (opts.tabId && opts.cwd) await registerTab(opts.cwd, opts.tabId, opts.command)
    const target = trustTarget(opts)
    if (target && opts.cwd && isTrustedProject(opts.cwd)) await writeTrust(opts.cwd, target)
    const { launch, env } = await launchFor(opts.command, args, {
      ...process.env,
      ...startupTrimEnv(opts.command),
      ...orientation.env,
      ...agy.env,
      ...(opts.tabId ? { LGRASS_TAB_ID: opts.tabId } : {}),
      ...(PROJECT_DIR_COMMANDS.has(opts.command) && opts.cwd
        ? { LGRASS_PROJECT_DIR: opts.cwd }
        : {}),
      ...(configDir ? { CLAUDE_CONFIG_DIR: configDir } : {})
    })
    if ('error' in launch) return { error: launch.error }
    // Applied after launchFor, because on Windows it rebuilds PATH from the registry and would drop the shim dir.
    const child =
      opts.command === SHELL_COMMAND
        ? shellTabLaunch(launch.file, launch.args, env, homedir())
        : { args: launch.args, env }
    const proc = pty.spawn(launch.file, child.args, {
      name: 'xterm-256color',
      cols: opts.cols ?? 80,
      rows: opts.rows ?? 24,
      cwd: opts.cwd ?? homedir(),
      env: child.env as Record<string, string>
    })

    proc.onData((data) => {
      send(getSender(), 'pty:data', { id, data })
    })

    proc.onExit(({ exitCode, signal }) => {
      if (!shuttingDown) send(getSender(), 'pty:exit', { id, exitCode, signal })
      shells.delete(id)
      // The tab's browser lives only as long as its shell, which is the tab id the web commands arrive with.
      if (opts.tabId) destroyInstance(opts.tabId)
    })

    shells.set(id, {
      process: proc,
      command: opts.command,
      tabId: opts.tabId,
      cwd: opts.cwd ?? homedir(),
      lastInputAt: 0
    })
    return { id }
  })

  ipcMain.on('pty:write', (_event, { id, data }: { id: string; data: string }) => {
    const shell = shells.get(id)
    if (!shell) return
    const kind = classifyInput(data)
    if (kind === 'typing') shell.lastInputAt = Date.now()
    else if (kind === 'enter') shell.lastInputAt = 0
    shell.process.write(data)
  })

  ipcMain.on(
    'pty:resize',
    (_event, { id, cols, rows }: { id: string; cols: number; rows: number }) => {
      shells.get(id)?.process.resize(cols, rows)
    }
  )

  ipcMain.on('pty:kill', (_event, { id }: { id: string }) => {
    shells.get(id)?.process.kill()
    shells.delete(id)
  })

  ipcMain.on('pty:graceful-close', (_event, { id }: { id: string }) => {
    void closeShell(id)
  })
}

export interface TabShell {
  command: string
  cwd: string
  humanIsTyping: () => boolean
  type: (text: string) => void
}

export function findShellByTab(tabId: string): TabShell | undefined {
  for (const shell of shells.values()) {
    if (shell.tabId !== tabId) continue
    return {
      command: shell.command,
      cwd: shell.cwd,
      humanIsTyping: () => Date.now() - shell.lastInputAt < HUMAN_QUIET_MS,
      type: (text) => shell.process.write(text)
    }
  }
  return undefined
}

export function killAllShells(): void {
  for (const shell of shells.values()) shell.process.kill()
  shells.clear()
}

// Includes shells still stopping, since their agent may still be running.
export function liveShellCwds(): string[] {
  return [...shells.values()].map((shell) => shell.cwd)
}

export function hasGracefulShells(): boolean {
  return [...shells.values()].some((shell) => gracefulClose(shell.command))
}

// A process exit during shutdown is not forwarded, because the renderer closes a tab on exit and would
// autosave a layout without it.
export function beginShutdown(): void {
  shuttingDown = true
}

export async function stopGracefulShells(): Promise<void> {
  const ids = [...shells].flatMap(([id, shell]) => (gracefulClose(shell.command) ? [id] : []))
  await Promise.all(ids.map(stopShell))
}

function closeShell(id: string): Promise<void> {
  const shell = shells.get(id)
  if (shell && !gracefulClose(shell.command)) {
    shell.process.kill()
    shells.delete(id)
    return Promise.resolve()
  }
  return stopShell(id)
}

function stopShell(id: string): Promise<void> {
  const inFlight = stopping.get(id)
  if (inFlight) return inFlight
  const shell = shells.get(id)
  if (!shell) return Promise.resolve()
  const stopped = new Promise<void>((resolve) => {
    const deadline = Date.now() + INTERRUPT_EXIT_WAIT_MS
    const interrupt = (): void => {
      const live = shells.get(id)
      if (!live) return resolve()
      if (Date.now() >= deadline) {
        live.process.kill()
        shells.delete(id)
        return resolve()
      }
      live.process.write(INTERRUPT)
      setTimeout(interrupt, INTERRUPT_INTERVAL_MS)
    }
    interrupt()
  }).finally(() => stopping.delete(id))
  stopping.set(id, stopped)
  return stopped
}
