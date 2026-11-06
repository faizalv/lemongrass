// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import type {
  SSHFileEntry,
  SSHHomeResult,
  SSHListResult,
  SSHSessionEvent,
  SSHSessionReply,
  SSHSessionRequest,
  SSHTransferStart
} from '../../../../../preload/types'
import { describeError } from '../../../vaultSession'
import { joinRemote } from './remotePath'

// The Files drawer's state lives here per server tab, not in the component, because moving a
// tab to another pane remounts its body; the shown folder, the open drawer and running transfers
// must survive that. It ends with the tab, in stopFiles.

export interface FileTransfer {
  id: string
  name: string
  direction: 'upload' | 'download'
  // The remote folder an upload lands in, so its listing refreshes when the upload is done.
  dir: string
  done: number
  total: number
  state: 'running' | 'done' | 'failed'
  error: string
}

export interface FilesState {
  alias: string
  open: boolean
  path: string | null
  entries: SSHFileEntry[]
  loading: boolean
  loaded: boolean
  // Why the folder cannot be shown at all; a failed action on a shown folder is a notice instead.
  blocked: 'notLoggedIn' | 'error' | null
  blockedText: string
  notice: string
  transfers: FileTransfer[]
  // Acting as root through passwordless sudo. Never remembered, so a new tab starts as the user.
  sudo: boolean
  // Why sudo was just turned off, shown once the folder reloads as the normal user.
  sudoDropped: string
}

const DONE_LINGER_MS = 2500

// The helper prefixes its errors with its package name, which says nothing to the human.
function plain(error: string): string {
  return error.replace(/^sshsession: /, '')
}

const states = reactive<Record<string, FilesState>>({})
const unsubscribe = new Map<string, () => void>()

export function filesOf(tabId: string, alias: string): FilesState {
  if (!states[tabId]) {
    states[tabId] = {
      alias,
      open: false,
      path: null,
      entries: [],
      loading: false,
      loaded: false,
      blocked: null,
      blockedText: '',
      notice: '',
      transfers: [],
      sudo: false,
      sudoDropped: ''
    }
  }
  return states[tabId]
}

function transferOf(s: FilesState, id: string): FileTransfer {
  let t = s.transfers.find((x) => x.id === id)
  if (!t) {
    // An event can arrive before the reply that names the transfer; the reply fills it in.
    t = {
      id,
      name: '',
      direction: 'upload',
      dir: '',
      done: 0,
      total: 0,
      state: 'running',
      error: ''
    }
    s.transfers.push(t)
  }
  return t
}

function dropTransfer(s: FilesState, id: string): void {
  s.transfers = s.transfers.filter((t) => t.id !== id)
}

function onEvent(tabId: string, ev: SSHSessionEvent): void {
  const s = states[tabId]
  if (!s) return
  if (ev.event === 'exit') {
    for (const t of s.transfers) {
      if (t.state === 'running') Object.assign(t, { state: 'failed', error: plain(ev.error) })
    }
    return
  }
  if (ev.event === 'failed' && ev.code === 'cancelled') {
    dropTransfer(s, ev.transfer)
    return
  }
  const t = transferOf(s, ev.transfer)
  if (ev.event === 'progress') {
    t.done = ev.done
    t.total = ev.total
  } else if (ev.event === 'failed') {
    t.state = 'failed'
    t.error = plain(ev.error)
  } else {
    t.state = 'done'
    t.done = t.total
    if (t.direction === 'upload' && t.dir === s.path) void load(tabId, s.path)
    setTimeout(() => {
      const now = states[tabId]
      if (now) dropTransfer(now, t.id)
    }, DONE_LINGER_MS)
  }
}

function listen(tabId: string): void {
  if (unsubscribe.has(tabId)) return
  unsubscribe.set(
    tabId,
    window.api.ssh.session.onEvent(tabId, (ev) => onEvent(tabId, ev))
  )
}

async function settle<T>(run: () => Promise<SSHSessionReply<T>>): Promise<SSHSessionReply<T>> {
  try {
    return await run()
  } catch (err) {
    return { ok: false, error: describeError(err) }
  }
}

export function transferRunning(s: FilesState): boolean {
  return s.transfers.some((t) => t.state === 'running')
}

function sudoProblem(s: FilesState, reply: { error: string; code?: string }): string | null {
  if (reply.code === 'sudoPassword') {
    return `sudo on ${s.alias} asks for a password, so Files cannot use it.`
  }
  if (reply.code === 'sudoRefused' || reply.code === 'noSftpServer') {
    return `Files cannot act as root on ${s.alias}: ${plain(reply.error)}`
  }
  return null
}

// A helper started with --sudo that cannot act as root falls back to the normal user: main
// restarts it without the flag and the folder reloads. Returns whether that happened.
async function dropSudo(
  tabId: string,
  reply: { error: string; code?: string },
  reload: boolean
): Promise<boolean> {
  const s = states[tabId]
  const problem = s?.sudo ? sudoProblem(s, reply) : null
  if (!s || !problem) return false
  s.sudo = false
  s.sudoDropped = problem
  await settle(async () => {
    await window.api.ssh.session.setSudo(tabId, false)
    return { ok: true, result: null }
  })
  if (reload) void load(tabId)
  return true
}

async function call<T>(
  tabId: string,
  req: SSHSessionRequest,
  reloadOnSudo = true
): Promise<SSHSessionReply<T>> {
  const s = states[tabId]
  listen(tabId)
  const reply = await settle(() => window.api.ssh.session.request<T>(tabId, s.alias, req))
  if (!reply.ok && (await dropSudo(tabId, reply, reloadOnSudo))) {
    return { ok: false, error: s.sudoDropped, code: reply.code }
  }
  return reply
}

export async function setSudo(tabId: string, on: boolean): Promise<void> {
  const s = states[tabId]
  if (!s || s.sudo === on || transferRunning(s)) return
  s.sudo = on
  s.sudoDropped = ''
  s.notice = ''
  const switched = await settle(async () => {
    await window.api.ssh.session.setSudo(tabId, on)
    return { ok: true, result: null }
  })
  if (!switched.ok) {
    s.sudo = !on
    s.notice = switched.error
    return
  }
  await load(tabId, s.path)
}

// Records a failed reply on the state: the login wall when the master is missing, otherwise a
// notice above the list. Returns whether the reply was ok.
function accept(s: FilesState, reply: SSHSessionReply<unknown>, what = ''): boolean {
  if (reply.ok) return true
  // The reload after a sudo fallback shows the reason.
  if (s.sudoDropped) return false
  if (reply.code === 'notLoggedIn') {
    s.blocked = 'notLoggedIn'
    return false
  }
  s.notice = what ? `${what}: ${plain(reply.error)}` : plain(reply.error)
  return false
}

export async function load(tabId: string, path?: string | null): Promise<void> {
  const s = states[tabId]
  if (!s) return
  s.loading = true
  try {
    let target = path ?? s.path
    if (!target) {
      const home = await call<SSHHomeResult>(tabId, { op: 'home' }, false)
      if (!home.ok) return sudoProblem(s, home) ? load(tabId, path) : block(s, home)
      target = home.result.path
    }
    const listed = await call<SSHListResult>(tabId, { op: 'list', path: target }, false)
    if (!listed.ok) {
      if (sudoProblem(s, listed)) return load(tabId, target)
      if (s.loaded && listed.code !== 'notLoggedIn') s.notice = plain(listed.error)
      else block(s, listed)
      return
    }
    s.path = listed.result.path
    s.entries = listed.result.entries
    s.loaded = true
    s.blocked = null
    s.notice = s.sudoDropped
    s.sudoDropped = ''
  } finally {
    s.loading = false
  }
}

function block(s: FilesState, reply: { error: string; code?: string }): void {
  s.blocked = reply.code === 'notLoggedIn' ? 'notLoggedIn' : 'error'
  s.blockedText = plain(reply.error)
  s.sudoDropped = ''
}

export async function makeFolder(tabId: string, name: string): Promise<boolean> {
  const s = states[tabId]
  if (!s?.path) return false
  const reply = await call(tabId, { op: 'mkdir', path: joinRemote(s.path, name) })
  if (!accept(s, reply, name)) return false
  await load(tabId)
  return true
}

export async function renameEntry(tabId: string, from: string, to: string): Promise<boolean> {
  const s = states[tabId]
  if (!s?.path) return false
  const reply = await call(tabId, {
    op: 'rename',
    from: joinRemote(s.path, from),
    to: joinRemote(s.path, to)
  })
  if (!accept(s, reply, from)) return false
  await load(tabId)
  return true
}

export async function removeEntry(tabId: string, name: string): Promise<void> {
  const s = states[tabId]
  if (!s?.path) return
  const reply = await call(tabId, { op: 'remove', path: joinRemote(s.path, name) })
  if (accept(s, reply, name)) await load(tabId)
}

function startText(name: string, reply: { error: string; code?: string }): string {
  if (reply.code === 'exists') return `${name} already exists there.`
  if (reply.code === 'isDir') return `${name} is a folder. Only files transfer for now.`
  return `${name}: ${plain(reply.error)}`
}

function track(s: FilesState, start: SSHTransferStart, direction: FileTransfer['direction']): void {
  const t = transferOf(s, start.transfer)
  t.name = start.name
  t.direction = direction
  t.dir = direction === 'upload' ? (s.path ?? '') : ''
  if (!t.total) t.total = start.total
}

export async function uploadFiles(tabId: string, locals: string[]): Promise<void> {
  const s = states[tabId]
  if (!s?.path) return
  listen(tabId)
  const dir = s.path
  for (const local of locals) {
    const name = local.split(/[\\/]/).pop() ?? local
    const reply = await settle(() => window.api.ssh.session.upload(tabId, s.alias, local, dir))
    if (reply.ok) track(s, reply.result, 'upload')
    else if (await dropSudo(tabId, reply, true)) return
    else if (reply.code === 'notLoggedIn') s.blocked = 'notLoggedIn'
    else s.notice = startText(name, reply)
  }
}

export async function downloadEntry(tabId: string, name: string): Promise<void> {
  const s = states[tabId]
  if (!s?.path) return
  listen(tabId)
  const remote = joinRemote(s.path, name)
  const reply = await settle(() => window.api.ssh.session.download(tabId, s.alias, remote))
  if (!reply.ok) {
    if (await dropSudo(tabId, reply, true)) return
    if (reply.code === 'notLoggedIn') s.blocked = 'notLoggedIn'
    else s.notice = startText(name, reply)
  } else if (reply.result) track(s, reply.result, 'download')
}

export async function cancelTransfer(tabId: string, id: string): Promise<void> {
  const s = states[tabId]
  if (!s) return
  const reply = await call(tabId, { op: 'cancel', transfer: id })
  // The transfer may have finished meanwhile; its own event settles the row either way.
  if (!reply.ok) s.notice = plain(reply.error)
}

export function dismissTransfer(tabId: string, id: string): void {
  const s = states[tabId]
  if (s) dropTransfer(s, id)
}

// Called when the server tab closes: ends the helper and forgets the drawer.
export function stopFiles(tabId: string): void {
  window.api.ssh.session.stop(tabId)
  unsubscribe.get(tabId)?.()
  unsubscribe.delete(tabId)
  delete states[tabId]
}
