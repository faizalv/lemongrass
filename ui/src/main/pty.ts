import { ipcMain, WebContents } from 'electron'
import { homedir } from 'os'
import * as pty from 'node-pty'
import { randomUUID } from 'crypto'
import { execFile } from 'child_process'
import { promisify } from 'util'

const execFileAsync = promisify(execFile)

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

const shells = new Map<string, pty.IPty>()

// Appends a knowledge TOC to a `claude` spawn's args via
// --append-system-prompt, shelling out to `lgrass knowledge toc` in the
// target cwd. Fails soft: if `lgrass` isn't on PATH, the cwd isn't a
// registered project, or the project has no knowledge yet, the shell
// still spawns, just without the extra prompt.
async function withKnowledgeToc(opts: SpawnOptions): Promise<string[]> {
  const args = opts.args ?? []
  if (opts.command !== 'claude') return args

  try {
    const { stdout } = await execFileAsync('lgrass', ['knowledge', 'toc'], {
      cwd: opts.cwd ?? homedir()
    })
    const toc = stdout.trim()
    if (!toc) return args
    return [...args, '--append-system-prompt', toc]
  } catch (err) {
    console.error('lgrass knowledge toc failed, spawning claude without injected knowledge:', err)
    return args
  }
}

// proc.onData/onExit fire on the child process's own timing, not the
// window's -- a shell can still be emitting output while the window is
// mid-teardown, when getSender() may return a wrapper whose underlying
// native object is already gone. .send() on that throws "Object has been
// destroyed", so every send here goes through this guard.
function send(sender: WebContents | undefined, channel: string, payload: unknown): void {
  if (sender && !sender.isDestroyed()) sender.send(channel, payload)
}

export function registerPtyHandlers(getSender: () => WebContents | undefined): void {
  ipcMain.handle('pty:spawn', async (_event, opts: SpawnOptions) => {
    const id = randomUUID()
    const args = await withKnowledgeToc(opts)
    const proc = pty.spawn(opts.command, args, {
      name: 'xterm-256color',
      cols: opts.cols ?? 80,
      rows: opts.rows ?? 24,
      cwd: opts.cwd ?? homedir(),
      env: process.env as Record<string, string>
    })

    proc.onData((data) => {
      send(getSender(), 'pty:data', { id, data })
    })

    proc.onExit(({ exitCode, signal }) => {
      send(getSender(), 'pty:exit', { id, exitCode, signal })
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
