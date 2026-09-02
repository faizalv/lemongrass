import { ipcMain, WebContents } from 'electron'
import { homedir } from 'os'
import * as pty from 'node-pty'
import { randomUUID } from 'crypto'

// PTY is display-only, never a control-signal source. Every shell spawns
// an agent binary directly (e.g. `claude`), never a login shell -- no
// `cd`/arbitrary commands as a first-class surface.

interface SpawnOptions {
  /** The agent binary to spawn, e.g. "claude". Never a shell like bash/zsh. */
  command: string
  /** Extra argv passed to the agent binary, e.g. ["--append-system-prompt", "..."]. */
  args?: string[]
  /** Working directory -- the project the shell is scoped to. */
  cwd?: string
  cols?: number
  rows?: number
}

// No system-prompt injection here: Claude Code already loads a project's
// own CLAUDE.md itself the moment it starts in that cwd, so re-injecting
// it via --append-system-prompt would just duplicate it into context.
// That flag is reserved for phase 2, once `lgrass` exists to add its own
// knowledge on top of what the agent CLI already loads natively -- see
// the PRD's Phase 2 section.

const shells = new Map<string, pty.IPty>()

export function registerPtyHandlers(getSender: () => WebContents | undefined): void {
  ipcMain.handle('pty:spawn', (_event, opts: SpawnOptions) => {
    const id = randomUUID()
    const proc = pty.spawn(opts.command, opts.args ?? [], {
      name: 'xterm-256color',
      cols: opts.cols ?? 80,
      rows: opts.rows ?? 24,
      cwd: opts.cwd ?? homedir(),
      env: process.env as Record<string, string>
    })

    proc.onData((data) => {
      getSender()?.send('pty:data', { id, data })
    })

    proc.onExit(({ exitCode, signal }) => {
      getSender()?.send('pty:exit', { id, exitCode, signal })
      shells.delete(id)
    })

    shells.set(id, proc)
    return { id }
  })

  ipcMain.on('pty:write', (_event, { id, data }: { id: string; data: string }) => {
    shells.get(id)?.write(data)
  })

  ipcMain.on(
    'pty:resize',
    (_event, { id, cols, rows }: { id: string; cols: number; rows: number }) => {
      shells.get(id)?.resize(cols, rows)
    }
  )

  ipcMain.on('pty:kill', (_event, { id }: { id: string }) => {
    shells.get(id)?.kill()
    shells.delete(id)
  })
}

export function killAllShells(): void {
  for (const proc of shells.values()) proc.kill()
  shells.clear()
}
