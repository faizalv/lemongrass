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

// The Files layer's state lives here per server tab, not in the component, because moving a
// tab to another pane remounts its body; the shown folder, its history, the clipboard and running
// transfers must survive that. It ends with the tab, in stopFiles.

export type FilesViewKind = 'detailed' | 'tile' | 'basic'

export interface FilesClipboard {
  path: string
  name: string
  mode: 'move' | 'copy'
}

export interface FileTransfer {
  id: string
  name: string
  direction: 'upload' | 'download'
  // The remote folder an upload lands in, so its listing refreshes when the upload is done.
  dir: string
  done: number
  // 0 while unknown, such as a folder streamed as a tar.gz.
  total: number
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
  // The layer's own folder history for Back and Forward.
  history: string[]
  historyAt: number
  view: FilesViewKind
  clipboard: FilesClipboard | null
  // What a slow operation is doing, set only once it has run past SLOW_AFTER_MS.
  slow: string
  // Acting as root through passwordless sudo. Never remembered, so a new tab starts as the user.
  sudo: boolean
  // Why sudo was just turned off, shown once the folder reloads as the normal user.
  sudoDropped: string
}

const SLOW_AFTER_MS = 300

// A new tab opens in the view last picked in any tab, for this run of the app.
let lastView: FilesViewKind = 'detailed'

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
      history: [],
      historyAt: -1,
      view: lastView,
      clipboard: null,
      slow: '',
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
      total: 0
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
    if (s.transfers.length) s.notice = `Transfers stopped: ${plain(ev.error)}`
    s.transfers = []
    return
  }
  if (ev.event === 'failed') {
    const name = s.transfers.find((t) => t.id === ev.transfer)?.name
    dropTransfer(s, ev.transfer)
    if (ev.code !== 'cancelled') s.notice = `${name || 'A transfer'}: ${plain(ev.error)}`
    return
  }
  const t = transferOf(s, ev.transfer)
  if (ev.event === 'progress') {
    t.done = ev.done
    t.total = ev.total
    return
  }
  dropTransfer(s, t.id)
  if (t.direction === 'upload' && t.dir === s.path) void load(tabId, s.path)
}

// Runs an operation, publishing its text as the layer's status only once it has taken longer
// than SLOW_AFTER_MS, so a quick one never flashes a status.
async function slowly<T>(s: FilesState, text: string, run: () => Promise<T>): Promise<T> {
  const timer = setTimeout(() => (s.slow = text), SLOW_AFTER_MS)
  try {
    return await run()
  } finally {
    clearTimeout(timer)
    if (s.slow === text) s.slow = ''
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
  return s.transfers.length > 0
}

export function setView(tabId: string, view: FilesViewKind): void {
  const s = states[tabId]
  if (s) s.view = view
  lastView = view
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
  // The layer reloads only when it has shown a folder; a remote file tab may be the only user.
  if (reload && s.loaded) void load(tabId)
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

// A request from outside the layer, such as a remote file tab, through the same server tab's
// helper, so it follows that tab's root mode and its fallback.
export function serverRequest<T>(
  tabId: string,
  alias: string,
  req: SSHSessionRequest
): Promise<SSHSessionReply<T>> {
  filesOf(tabId, alias)
  return call<T>(tabId, req)
}

export function actingAsRoot(tabId: string): boolean {
  return states[tabId]?.sudo ?? false
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

// Lists a folder, the shown one when no path is given and home before anything was shown.
// Returns whether the folder is now shown.
export async function load(tabId: string, path?: string | null): Promise<boolean> {
  const s = states[tabId]
  if (!s) return false
  s.loading = true
  try {
    let target = path ?? s.path
    if (!target) {
      const home = await call<SSHHomeResult>(tabId, { op: 'home' }, false)
      if (!home.ok) {
        if (sudoProblem(s, home)) return load(tabId, path)
        block(s, home)
        return false
      }
      target = home.result.path
    }
    const listed = await slowly(s, `Opening ${target}`, () =>
      call<SSHListResult>(tabId, { op: 'list', path: target }, false)
    )
    if (!listed.ok) {
      if (sudoProblem(s, listed)) return load(tabId, target)
      if (s.loaded && listed.code !== 'notLoggedIn') s.notice = plain(listed.error)
      else block(s, listed)
      return false
    }
    s.path = listed.result.path
    s.entries = listed.result.entries
    s.loaded = true
    s.blocked = null
    s.notice = s.sudoDropped
    s.sudoDropped = ''
    return true
  } finally {
    s.loading = false
  }
}

// Goes to a folder and records it in the history, dropping any Forward entries.
export async function navigate(tabId: string, path: string | null): Promise<boolean> {
  const s = states[tabId]
  if (!s || !(await load(tabId, path)) || !s.path) return false
  if (s.history[s.historyAt] !== s.path) {
    s.history = [...s.history.slice(0, s.historyAt + 1), s.path]
    s.historyAt = s.history.length - 1
  }
  return true
}

export async function goHistory(tabId: string, step: -1 | 1): Promise<void> {
  const s = states[tabId]
  const at = (s?.historyAt ?? 0) + step
  if (!s || at < 0 || at >= s.history.length) return
  if (await load(tabId, s.history[at])) s.historyAt = at
}

function block(s: FilesState, reply: { error: string; code?: string }): void {
  s.blocked = reply.code === 'notLoggedIn' ? 'notLoggedIn' : 'error'
  s.blockedText = plain(reply.error)
  s.sudoDropped = ''
}

function replyText(name: string, reply: { error: string; code?: string }): string {
  if (reply.code === 'exists') return `${name} already exists.`
  return plain(reply.error)
}

// The name modal's actions return the error to show in the modal, or null once done. A login
// wall or a sudo fallback closes the modal, since the layer itself shows those.
async function modalOp(
  tabId: string,
  text: string,
  name: string,
  req: SSHSessionRequest
): Promise<string | null> {
  const s = states[tabId]
  if (!s?.path) return null
  const reply = await slowly(s, text, () => call(tabId, req))
  if (reply.ok) {
    await load(tabId)
    return null
  }
  if (s.sudoDropped) return null
  if (reply.code === 'notLoggedIn') {
    s.blocked = 'notLoggedIn'
    return null
  }
  return replyText(name, reply)
}

export function createEntry(
  tabId: string,
  kind: 'folder' | 'file',
  name: string
): Promise<string | null> {
  const path = joinRemote(states[tabId]?.path ?? '/', name)
  return modalOp(tabId, `Creating ${name}`, name, {
    op: kind === 'folder' ? 'mkdir' : 'touch',
    path
  })
}

export function renameEntry(tabId: string, from: string, to: string): Promise<string | null> {
  const dir = states[tabId]?.path ?? '/'
  return modalOp(tabId, `Renaming ${from}`, to, {
    op: 'rename',
    from: joinRemote(dir, from),
    to: joinRemote(dir, to)
  })
}

export async function removeEntry(tabId: string, name: string): Promise<void> {
  const s = states[tabId]
  if (!s?.path) return
  const path = joinRemote(s.path, name)
  const reply = await slowly(s, `Deleting ${name}`, () => call(tabId, { op: 'remove', path }))
  if (accept(s, reply, name)) await load(tabId)
}

export function setClipboard(tabId: string, name: string, mode: FilesClipboard['mode']): void {
  const s = states[tabId]
  if (s?.path) s.clipboard = { path: joinRemote(s.path, name), name, mode }
}

// Moves or copies the clipboard's entry into the shown folder under the same name.
export async function paste(tabId: string): Promise<void> {
  const s = states[tabId]
  const clip = s?.clipboard
  if (!s?.path || !clip) return
  const to = joinRemote(s.path, clip.name)
  const text = `${clip.mode === 'move' ? 'Moving' : 'Copying'} ${clip.name}`
  const reply = await slowly(s, text, () => call(tabId, { op: clip.mode, from: clip.path, to }))
  if (reply.ok) {
    if (clip.mode === 'move') s.clipboard = null
    await load(tabId)
  } else if (reply.code === 'notLoggedIn') {
    s.blocked = 'notLoggedIn'
  } else if (!s.sudoDropped) {
    s.notice = `${clip.name}: ${replyText(clip.name, reply)}`
  }
}

function startText(name: string, reply: { error: string; code?: string }): string {
  if (reply.code === 'exists') return `${name} already exists there.`
  if (reply.code === 'isDir') return `${name} is a folder. Only files upload.`
  return `${name}: ${plain(reply.error)}`
}

function track(s: FilesState, start: SSHTransferStart, direction: FileTransfer['direction']): void {
  const t = transferOf(s, start.transfer)
  t.name = start.name
  t.direction = direction
  t.dir = direction === 'upload' ? (s.path ?? '') : ''
  if (!t.total) t.total = start.total
}

export async function uploadPicked(tabId: string): Promise<void> {
  let paths: string[] = []
  try {
    paths = await window.api.ssh.session.pickUploads()
  } catch (err) {
    const s = states[tabId]
    if (s) s.notice = describeError(err)
  }
  if (paths.length) await uploadFiles(tabId, paths)
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

// A folder downloads as one tar.gz the server streams; the save dialog is main's.
export async function downloadEntry(tabId: string, entry: SSHFileEntry): Promise<void> {
  const s = states[tabId]
  if (!s?.path) return
  listen(tabId)
  const name = entry.name
  const remote = joinRemote(s.path, name)
  const reply = await settle(() =>
    window.api.ssh.session.download(tabId, s.alias, remote, entry.dir)
  )
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

// Called when the server tab closes: ends the helper and forgets the drawer.
export function stopFiles(tabId: string): void {
  window.api.ssh.session.stop(tabId)
  unsubscribe.get(tabId)?.()
  unsubscribe.delete(tabId)
  delete states[tabId]
}
