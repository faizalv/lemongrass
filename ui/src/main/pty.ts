import { ipcMain, WebContents } from 'electron'
import { homedir } from 'os'
import * as pty from 'node-pty'
import { randomUUID } from 'crypto'

// Every shell spawns an agent binary directly, never a login shell. Codex
// receives repeated Ctrl+C during a graceful tab or app shutdown, so it can
// run its SessionEnd hook before the PTY is force-killed.

interface SpawnOptions {
  /** The agent binary to spawn, e.g. "claude". Never a shell like bash/zsh. */
  command: string
  /** Extra argv passed to the agent binary, e.g. ["--append-system-prompt", "..."]. */
  args?: string[]
  /** Working directory: the project the shell is scoped to. */
  cwd?: string
  /** Workspace tab id, exposed to the agent process so its session hooks can tie a session to this tab. */
  tabId?: string
  cols?: number
  rows?: number
}

interface ManagedShell {
  process: pty.IPty
  command: string
}

const shells = new Map<string, ManagedShell>()
const stopping = new Map<string, Promise<void>>()
let shuttingDown = false
// Ctrl+C clears a draft, interrupts a running turn, and quits on a repeat press once the composer is idle.
const CODEX_INTERRUPT = '\x03'
const CODEX_INTERRUPT_INTERVAL_MS = 400
const CODEX_EXIT_WAIT_MS = 3_000

// proc.onData/onExit fire on the child process's own timing, not the
// window's. A shell can still be emitting output while the window is
// mid-teardown, when getSender() may return a wrapper whose underlying
// native object is already gone. .send() on that throws "Object has been
// destroyed", so every send here goes through this guard.
function send(sender: WebContents | undefined, channel: string, payload: unknown): void {
  if (sender && !sender.isDestroyed()) sender.send(channel, payload)
}

export function registerPtyHandlers(getSender: () => WebContents | undefined): void {
  ipcMain.handle('pty:spawn', async (_event, opts: SpawnOptions) => {
    const id = randomUUID()
    const args = opts.args ?? []
    const proc = pty.spawn(opts.command, args, {
      name: 'xterm-256color',
      cols: opts.cols ?? 80,
      rows: opts.rows ?? 24,
      cwd: opts.cwd ?? homedir(),
      env: {
        ...(process.env as Record<string, string>),
        ...(opts.tabId ? { LGRASS_TAB_ID: opts.tabId } : {})
      }
    })

    proc.onData((data) => {
      send(getSender(), 'pty:data', { id, data })
    })

    proc.onExit(({ exitCode, signal }) => {
      if (!shuttingDown) send(getSender(), 'pty:exit', { id, exitCode, signal })
      shells.delete(id)
    })

    shells.set(id, { process: proc, command: opts.command })
    return { id }
  })

  ipcMain.on('pty:write', (_event, { id, data }: { id: string; data: string }) => {
    shells.get(id)?.process.write(data)
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

export function killAllShells(): void {
  for (const shell of shells.values()) shell.process.kill()
  shells.clear()
}

export function hasCodexShells(): boolean {
  return [...shells.values()].some((shell) => shell.command === 'codex')
}

// A process exit during shutdown is not forwarded, because the renderer closes a tab on exit and would
// autosave a layout without it.
export function beginShutdown(): void {
  shuttingDown = true
}

export async function stopCodexShells(): Promise<void> {
  const codexIDs = [...shells].flatMap(([id, shell]) => (shell.command === 'codex' ? [id] : []))
  await Promise.all(codexIDs.map(stopShell))
}

function closeShell(id: string): Promise<void> {
  const shell = shells.get(id)
  if (shell && shell.command !== 'codex') {
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
    const deadline = Date.now() + CODEX_EXIT_WAIT_MS
    const interrupt = (): void => {
      const live = shells.get(id)
      if (!live) return resolve()
      if (Date.now() >= deadline) {
        live.process.kill()
        shells.delete(id)
        return resolve()
      }
      live.process.write(CODEX_INTERRUPT)
      setTimeout(interrupt, CODEX_INTERRUPT_INTERVAL_MS)
    }
    interrupt()
  }).finally(() => stopping.delete(id))
  stopping.set(id, stopped)
  return stopped
}
