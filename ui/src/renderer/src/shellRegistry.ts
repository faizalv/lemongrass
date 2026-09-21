import { reactive } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'

// Owns each shell's xterm instance and PTY by tab id, so a shell keeps running
// while its tab is hidden, moved to another pane, or its project is not shown.
// Raw bytes only in both directions. PTY is display-only, never a control-signal source.

export interface ShellSpec {
  id: string
  command: string
  cwd?: string
}

interface ShellSession {
  spec: ShellSpec
  term: Terminal
  fit: FitAddon
  host: HTMLDivElement
  opened: boolean
  disposed: boolean
  shellId?: string
}

const sessions = new Map<string, ShellSession>()
const byShellId = new Map<string, ShellSession>()
const bufferedData = new Map<string, string[]>()
const earlyExits = new Set<string>()
const initialArgs = new Map<string, string[]>()
// Escape followed by carriage return, which the agent CLI reads as a newline in its input.
const NEWLINE_SEQUENCE = '\x1b\r'
let exitHandler: ((tabId: string) => void) | undefined
let listening = false

export const shellTitles = reactive<Record<string, string>>({})

export function onShellExit(handler: (tabId: string) => void): void {
  exitHandler = handler
}

export function setShellArgs(tabId: string, args: string[]): void {
  initialArgs.set(tabId, args)
}

function cssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

function listen(): void {
  if (listening) return
  listening = true
  window.api.pty.onData(({ id, data }) => {
    const session = byShellId.get(id)
    if (session) {
      session.term.write(data)
      return
    }
    bufferedData.set(id, [...(bufferedData.get(id) ?? []), data])
  })
  window.api.pty.onExit(({ id }) => {
    const session = byShellId.get(id)
    if (!session) {
      earlyExits.add(id)
      return
    }
    byShellId.delete(id)
    session.shellId = undefined
    exitHandler?.(session.spec.id)
  })
}

function createSession(spec: ShellSpec): ShellSession {
  const term = new Terminal({
    fontFamily: cssVar('--font-mono'),
    fontSize: 13,
    fontWeight: 400,
    fontWeightBold: 600,
    cursorStyle: 'bar',
    cursorBlink: false,
    theme: {
      background: cssVar('--color-surface-1'),
      foreground: cssVar('--color-fg-primary'),
      cursor: cssVar('--color-amber'),
      cursorAccent: cssVar('--color-surface-1'),
      selectionBackground: cssVar('--color-amber-muted'),
      black: cssVar('--color-gray-700'),
      red: cssVar('--color-coral-dim'),
      green: cssVar('--color-jade-dim'),
      yellow: cssVar('--color-amber-dim'),
      blue: cssVar('--color-sapphire-dim'),
      magenta: cssVar('--color-violet-dim'),
      cyan: cssVar('--color-teal-dim'),
      white: cssVar('--color-gray-100'),
      brightBlack: cssVar('--color-gray-500'),
      brightRed: cssVar('--color-coral'),
      brightGreen: cssVar('--color-jade'),
      brightYellow: cssVar('--color-amber'),
      brightBlue: cssVar('--color-sapphire'),
      brightMagenta: cssVar('--color-violet'),
      brightCyan: cssVar('--color-teal'),
      brightWhite: cssVar('--color-white')
    }
  })
  const fit = new FitAddon()
  term.loadAddon(fit)
  const host = document.createElement('div')
  host.style.height = '100%'
  host.style.width = '100%'
  const session: ShellSession = { spec, term, fit, host, opened: false, disposed: false }

  term.onTitleChange((title) => {
    if (title) shellTitles[spec.id] = title
  })
  term.onData((data) => {
    if (session.shellId) window.api.pty.write(session.shellId, data)
  })
  term.attachCustomKeyEventHandler((event) => {
    if (event.key !== 'Enter' || !event.shiftKey) return true
    if (event.type === 'keydown' && session.shellId) {
      window.api.pty.write(session.shellId, NEWLINE_SEQUENCE)
    }
    return false
  })
  return session
}

async function start(session: ShellSession): Promise<void> {
  const { spec, term } = session
  const args = initialArgs.get(spec.id)
  initialArgs.delete(spec.id)
  const { id } = await window.api.pty.spawn({
    command: spec.command,
    args,
    cwd: spec.cwd,
    cols: term.cols,
    rows: term.rows
  })
  if (session.disposed) {
    window.api.pty.kill(id)
    return
  }
  session.shellId = id
  byShellId.set(id, session)
  for (const chunk of bufferedData.get(id) ?? []) term.write(chunk)
  bufferedData.delete(id)
  if (earlyExits.delete(id)) {
    byShellId.delete(id)
    session.shellId = undefined
    exitHandler?.(spec.id)
  }
}

export function fitShell(tabId: string): void {
  const session = sessions.get(tabId)
  if (!session || session.host.clientWidth === 0 || session.host.clientHeight === 0) return
  session.fit.fit()
  if (session.shellId) window.api.pty.resize(session.shellId, session.term.cols, session.term.rows)
}

export function attachShell(spec: ShellSpec, container: HTMLElement): void {
  listen()
  let session = sessions.get(spec.id)
  if (!session) {
    session = createSession(spec)
    sessions.set(spec.id, session)
  }
  container.appendChild(session.host)
  if (!session.opened) {
    session.opened = true
    session.term.open(session.host)
    session.fit.fit()
    void start(session)
    return
  }
  fitShell(spec.id)
  session.term.refresh(0, session.term.rows - 1)
}

export function disposeShell(tabId: string): void {
  const session = sessions.get(tabId)
  if (!session) return
  sessions.delete(tabId)
  delete shellTitles[tabId]
  session.disposed = true
  if (session.shellId) {
    byShellId.delete(session.shellId)
    window.api.pty.kill(session.shellId)
  }
  session.term.dispose()
  session.host.remove()
}
