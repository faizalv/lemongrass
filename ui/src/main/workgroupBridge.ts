import { BrowserWindow, dialog, ipcMain } from 'electron'
import { chmodSync, mkdirSync, rmSync, writeFileSync } from 'fs'
import * as net from 'net'
import { homedir } from 'os'
import { join } from 'path'
import { randomUUID } from 'crypto'
import type { WorkgroupSpawnMember, WorkgroupSpawnResult } from '../preload/types'
import { handleNudge } from './tabNudge'

// The socket `lgrass workgroup create` talks to. A proposal shows the human a confirmation dialog and,
// once approved, returns a one-time token. Only that token spawns tabs, and it spawns exactly what was shown.

const MAX_COPILOTS = 5
const MAX_PROMPT_CHARS = 4000
const TOKEN_TTL_MS = 5 * 60_000
const SPAWN_ACK_TIMEOUT_MS = 15_000
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/
const LABEL = /^[A-Za-z0-9][A-Za-z0-9_-]{0,29}$/
const MODEL = /^[A-Za-z0-9][A-Za-z0-9._:/-]{0,79}$/
const SKILL = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/
const MAX_SKILLS = 9
const VENDORS = new Set(['claude', 'codex'])

interface Proposal {
  op: string
  project_path: string
  pilot_tab_id: string
  pilot_label: string
  group_name: string
  members: {
    tab_id: string
    label: string
    vendor: string
    model?: string
    prompt: string
    skills: string[]
  }[]
  token?: string
  tab_id?: string
}

interface Approved {
  projectPath: string
  pilotTabId: string
  members: WorkgroupSpawnMember[]
  expires: number
}

interface Reply {
  ok: boolean
  approved?: boolean
  token?: string
  error?: string
  typed?: boolean
  reason?: string
}

const approvals = new Map<string, Approved>()
const spawnWaiters = new Map<string, (result: WorkgroupSpawnResult) => void>()
let dialogQueue: Promise<unknown> = Promise.resolve()

export function workgroupSocketPath(): string {
  return join(homedir(), '.lemongrass', 'app.sock')
}

function validate(p: Proposal): string | null {
  if (typeof p.project_path !== 'string' || !p.project_path) return 'no project path'
  if (!UUID.test(p.pilot_tab_id ?? '')) return 'the pilot tab id is not valid'
  if (!LABEL.test(p.pilot_label ?? '')) return 'the pilot label is not valid'
  if (typeof p.group_name !== 'string' || !p.group_name) return 'no group name'
  if (!Array.isArray(p.members) || p.members.length < 1 || p.members.length > MAX_COPILOTS) {
    return `a group needs 1 to ${MAX_COPILOTS} copilots`
  }
  for (const m of p.members) {
    if (!UUID.test(m.tab_id ?? '')) return 'a copilot tab id is not valid'
    if (!LABEL.test(m.label ?? '')) return 'a copilot label is not valid'
    if (!VENDORS.has(m.vendor)) return 'a copilot vendor is not supported'
    if (m.model && !MODEL.test(m.model)) return 'a copilot model is not valid'
    if (
      !Array.isArray(m.skills) ||
      m.skills.length > MAX_SKILLS ||
      !m.skills.every((name) => typeof name === 'string' && SKILL.test(name))
    ) {
      return 'a copilot skill list is not valid'
    }
    if (typeof m.prompt !== 'string' || !m.prompt || m.prompt.length > MAX_PROMPT_CHARS) {
      return `a copilot prompt must be 1 to ${MAX_PROMPT_CHARS} characters`
    }
  }
  return null
}

function proposalText(p: Proposal): string {
  return p.members
    .map((m) => {
      const model = m.model ? `, model ${m.model}` : ''
      const skills = m.skills.length ? `Must load: ${m.skills.join(', ')}\n` : ''
      return `${m.label} (${m.vendor}${model})\n${skills}${m.prompt}`
    })
    .join('\n\n----\n\n')
}

function writeProposalFile(p: Proposal): string | null {
  const dir = join(homedir(), '.lemongrass', 'proposals')
  const path = join(dir, `${randomUUID()}.txt`)
  try {
    mkdirSync(dir, { recursive: true, mode: 0o700 })
    writeFileSync(path, proposalText(p), { mode: 0o600 })
    return path
  } catch {
    return null
  }
}

function describe(p: Proposal, file: string | null): { message: string; detail: string } {
  return {
    message: `The agent in tab "${p.pilot_label}" wants to start the workgroup "${p.group_name}" with ${p.members.length} co-pilot${p.members.length === 1 ? '' : 's'}.`,
    detail: file ? `Full text: ${file}` : 'The full text could not be written to a file.'
  }
}

function askHuman(window: BrowserWindow | undefined, p: Proposal): Promise<boolean> {
  const run = (): Promise<boolean> => {
    const file = writeProposalFile(p)
    const { message, detail } = describe(p, file)
    const options: Electron.MessageBoxOptions = {
      type: 'question',
      title: 'Approve workgroup',
      message,
      detail,
      buttons: ['Approve', 'Decline'],
      defaultId: 1,
      cancelId: 1,
      noLink: true
    }
    return (
      window && !window.isDestroyed()
        ? dialog.showMessageBox(window, options)
        : dialog.showMessageBox(options)
    )
      .then((result) => result.response === 0)
      .finally(() => {
        if (file) rmSync(file, { force: true })
      })
  }
  const answer = dialogQueue.then(run, run)
  dialogQueue = answer.catch(() => undefined)
  return answer
}

function pruneApprovals(): void {
  const now = Date.now()
  for (const [token, approved] of approvals) if (approved.expires < now) approvals.delete(token)
}

async function handlePropose(
  p: Proposal,
  getWindow: () => BrowserWindow | undefined
): Promise<Reply> {
  const problem = validate(p)
  if (problem) return { ok: false, error: problem }
  const approved = await askHuman(getWindow(), p)
  if (!approved) return { ok: true, approved: false }
  pruneApprovals()
  const token = randomUUID()
  approvals.set(token, {
    projectPath: p.project_path,
    pilotTabId: p.pilot_tab_id,
    members: p.members.map((m) => ({
      tabId: m.tab_id,
      label: m.label,
      vendor: m.vendor,
      model: m.model || undefined,
      prompt: m.prompt
    })),
    expires: Date.now() + TOKEN_TTL_MS
  })
  return { ok: true, approved: true, token }
}

async function handleSpawn(
  token: string,
  getWindow: () => BrowserWindow | undefined
): Promise<Reply> {
  pruneApprovals()
  const approved = approvals.get(token)
  if (!approved) return { ok: false, error: 'the approval is unknown or has expired' }
  approvals.delete(token)
  const window = getWindow()
  if (!window || window.isDestroyed())
    return { ok: false, error: 'the lemongrass window is not open' }

  const requestId = randomUUID()
  const result = await new Promise<WorkgroupSpawnResult>((resolve) => {
    const timer = setTimeout(() => {
      spawnWaiters.delete(requestId)
      resolve({ ok: false, error: 'the window did not answer in time' })
    }, SPAWN_ACK_TIMEOUT_MS)
    spawnWaiters.set(requestId, (r) => {
      clearTimeout(timer)
      resolve(r)
    })
    window.webContents.send('workgroup:spawn', {
      requestId,
      projectPath: approved.projectPath,
      pilotTabId: approved.pilotTabId,
      members: approved.members
    })
  })
  return result.ok ? { ok: true } : { ok: false, error: result.error ?? 'the spawn failed' }
}

function serveConnection(conn: net.Socket, getWindow: () => BrowserWindow | undefined): void {
  let buffer = ''
  conn.setEncoding('utf8')
  conn.setTimeout(11 * 60_000, () => conn.destroy())
  conn.on('error', () => conn.destroy())
  conn.on('data', (chunk: string) => {
    buffer += chunk
    if (buffer.length > 200_000) return void conn.destroy()
    const end = buffer.indexOf('\n')
    if (end === -1) return
    const line = buffer.slice(0, end)
    buffer = ''
    void respond(line, getWindow).then((reply) => conn.end(JSON.stringify(reply) + '\n'))
  })
}

async function respond(line: string, getWindow: () => BrowserWindow | undefined): Promise<Reply> {
  let request: Proposal
  try {
    request = JSON.parse(line)
  } catch {
    return { ok: false, error: 'the request is not valid JSON' }
  }
  try {
    if (request.op === 'propose') return await handlePropose(request, getWindow)
    if (request.op === 'spawn') return await handleSpawn(String(request.token ?? ''), getWindow)
    if (request.op === 'nudge')
      return await handleNudge(nudgeLgrassPath, String(request.tab_id ?? ''))
    return { ok: false, error: `unknown op ${String(request.op)}` }
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : String(err) }
  }
}

let server: net.Server | undefined
let nudgeLgrassPath: string | null = null

export function startWorkgroupBridge(
  getWindow: () => BrowserWindow | undefined,
  lgrassPath: string | null
): void {
  nudgeLgrassPath = lgrassPath
  ipcMain.on(
    'workgroup:spawn-result',
    (_event, payload: WorkgroupSpawnResult & { requestId: string }) => {
      const waiter = spawnWaiters.get(payload.requestId)
      if (!waiter) return
      spawnWaiters.delete(payload.requestId)
      waiter({ ok: payload.ok, error: payload.error })
    }
  )

  const path = workgroupSocketPath()
  mkdirSync(join(homedir(), '.lemongrass'), { recursive: true, mode: 0o700 })
  rmSync(path, { force: true })
  server = net.createServer((conn) => serveConnection(conn, getWindow))
  server.on('error', (err) => console.error('workgroup bridge:', err))
  server.listen(path, () => {
    try {
      chmodSync(path, 0o600)
    } catch (err) {
      console.error('workgroup bridge: chmod failed:', err)
    }
  })
}

export function stopWorkgroupBridge(): void {
  server?.close()
  server = undefined
  rmSync(workgroupSocketPath(), { force: true })
}
