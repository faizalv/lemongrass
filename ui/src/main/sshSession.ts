// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { spawn, type ChildProcessWithoutNullStreams } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { basename, isAbsolute, join, posix } from 'node:path'
import { app, BrowserWindow, dialog, ipcMain, type WebContents } from 'electron'
import type {
  SSHRelayRequest,
  SSHSessionReply,
  SSHSessionRequest,
  SSHTransferStart
} from '../preload/types'
import { ALIAS_PATTERN } from './sshHosts'
import { createRelayHub, type RelayHub } from './sshRelay'
import { relayStagePorts, removeStagesNow, sweepStaleStages } from './sshRelayStage'
import { createSessionLink, type SessionLink, type SessionMessage } from './sshSessionLink'
import { controlPath } from './sshTerminal'

// A server tab's file drawer talks to one `lgrassd ssh session` helper, keyed by the tab id, not
// by the component: a tab moved to another pane remounts its body and must keep its helper. The
// helper opens SFTP over the terminal's ControlMaster socket, so it never prompts for anything.

type Message = SessionMessage

interface Helper {
  proc: ChildProcessWithoutNullStreams
  link: SessionLink
  sender: WebContents
  stopped: boolean
}

const helpers = new Map<string, Helper>()
// Tabs whose drawer acts as root. Kept in memory only, so sudo is off each time a tab opens.
const sudoTabs = new Set<string>()
const watchedSenders = new WeakSet<WebContents>()
// The window each relay's source and destination tab live in, to start their helpers and send
// the destination the relay's events.
const relaySenders = new Map<string, WebContents>()
let relays: RelayHub | null = null
const STDERR_KEEP = 2000

const REQUEST_OPS = new Set<SSHSessionRequest['op']>([
  'home',
  'list',
  'mkdir',
  'rename',
  'remove',
  'touch',
  'move',
  'copy',
  'read',
  'write',
  'cancel'
])

function send(sender: WebContents, payload: Message): void {
  if (!sender.isDestroyed()) sender.send('ssh:session-event', payload)
}

function watchSender(sender: WebContents): void {
  if (watchedSenders.has(sender)) return
  watchedSenders.add(sender)
  sender.once('destroyed', () => {
    for (const [tabId, helper] of helpers) if (helper.sender === sender) stopSession(tabId)
    for (const [tabId, s] of relaySenders) if (s === sender) stopSession(tabId)
  })
}

function startHelper(
  lgrassdPath: string | null,
  tabId: string,
  alias: string,
  sender: WebContents
): Helper {
  const running = helpers.get(tabId)
  if (running) return running
  if (!lgrassdPath) throw new Error('ssh: lgrassd is not installed.')
  if (!ALIAS_PATTERN.test(alias)) throw new Error(`ssh: "${alias}" is not a valid server name.`)

  const args = ['ssh', 'session', alias, '--control-path', controlPath()]
  if (sudoTabs.has(tabId)) args.push('--sudo')
  const proc = spawn(lgrassdPath, args, {
    stdio: ['pipe', 'pipe', 'pipe'],
    windowsHide: true
  })
  const link = createSessionLink(
    (line) => {
      if (proc.stdin.writable) proc.stdin.write(line)
    },
    (event) => {
      if (!relays?.claim(tabId, event)) send(sender, { ...event, tabId })
    }
  )
  const helper: Helper = { proc, link, sender, stopped: false }
  let stderr = ''
  let ended = false

  const end = (error: string): void => {
    if (ended) return
    ended = true
    if (helpers.get(tabId) === helper) helpers.delete(tabId)
    link.close(error)
    relays?.helperEnded(tabId, error)
    if (!helper.stopped) send(sender, { tabId, event: 'exit', error })
  }

  proc.stdout.setEncoding('utf8')
  proc.stdout.on('data', (chunk: string) => link.feed(chunk))
  proc.stderr.setEncoding('utf8')
  proc.stderr.on('data', (chunk: string) => {
    stderr = (stderr + chunk).slice(-STDERR_KEEP)
  })
  // A write racing the helper's exit fails with EPIPE; the exit handler reports it.
  proc.stdin.on('error', () => undefined)
  proc.on('error', (err) => end(`ssh: the file helper did not start: ${err.message}`))
  proc.on('close', (code) => {
    const last = stderr.trim().split('\n').pop()
    end(last || `ssh: the file helper stopped (exit ${code ?? 'signal'}).`)
  })

  helpers.set(tabId, helper)
  watchSender(sender)
  return helper
}

// Ends the tab's helper without an exit event; the next request starts a new one.
function stopHelper(tabId: string): void {
  const helper = helpers.get(tabId)
  if (!helper) return
  helpers.delete(tabId)
  helper.stopped = true
  helper.link.close('ssh: the file helper was stopped.')
  helper.proc.stdin.end()
  helper.proc.kill()
}

export function stopSession(tabId: string): void {
  relays?.tabClosed(tabId)
  relaySenders.delete(tabId)
  stopHelper(tabId)
  sudoTabs.delete(tabId)
}

// Switching user means a new helper, since the sftp server runs as root or as the login user
// from the moment it starts.
function setSudo(tabId: string, on: boolean): void {
  if (sudoTabs.has(tabId) === on) return
  if (on) sudoTabs.add(tabId)
  else sudoTabs.delete(tabId)
  stopHelper(tabId)
}

export function stopAllSessions(): void {
  for (const tabId of [...helpers.keys()]) stopSession(tabId)
}

function absoluteRemote(path: string): string {
  if (!path.startsWith('/')) throw new Error('ssh: a remote path must be absolute.')
  return path
}

async function startTransfer(
  helper: Helper,
  body: Message,
  transfer: string,
  name: string
): Promise<SSHSessionReply<SSHTransferStart>> {
  const reply = await helper.link.request({ ...body, transfer })
  if (!reply.ok) return reply
  const total = (reply.result as { total?: unknown }).total
  return { ok: true, result: { transfer, name, total: typeof total === 'number' ? total : 0 } }
}

export function registerSSHSessionHandlers(lgrassdPath: string | null): void {
  const hub = createRelayHub({
    ...relayStagePorts,
    request: (tabId, alias, body) => {
      const sender = relaySenders.get(tabId) ?? helpers.get(tabId)?.sender
      if (!sender) throw new Error('ssh: the server tab is closed.')
      return startHelper(lgrassdPath, tabId, alias, sender).link.request(body)
    },
    emit: (event) => {
      const sender = relaySenders.get(event.tabId)
      if (sender) send(sender, event)
    }
  })
  relays = hub
  void sweepStaleStages()

  ipcMain.handle(
    'ssh:sessionRequest',
    (event, tabId: string, alias: string, req: SSHSessionRequest) => {
      if (!REQUEST_OPS.has(req.op)) throw new Error(`ssh: unknown file request ${req.op}.`)
      return startHelper(lgrassdPath, tabId, alias, event.sender).link.request({ ...req })
    }
  )

  // The remote target is the shown folder plus the local file's name; the helper refuses a taken
  // name with code exists rather than overwriting it.
  ipcMain.handle(
    'ssh:sessionUpload',
    (event, tabId: string, alias: string, local: string, remoteDir: string) => {
      if (!isAbsolute(local)) throw new Error('ssh: a local file must be an absolute path.')
      const helper = startHelper(lgrassdPath, tabId, alias, event.sender)
      const name = basename(local)
      const remote = posix.join(absoluteRemote(remoteDir), name)
      return startTransfer(helper, { op: 'upload', local, remote }, randomUUID(), name)
    }
  )

  // The local target is picked here in the host save dialog, so the renderer never names a local
  // path to write to. The dialog has already asked about replacing a file, so the helper replaces
  // it, and only once the download succeeds. A folder arrives as one tar.gz the server streams.
  ipcMain.handle(
    'ssh:sessionDownload',
    async (
      event,
      tabId: string,
      alias: string,
      remote: string,
      dir: boolean
    ): Promise<SSHSessionReply<SSHTransferStart | null>> => {
      const helper = startHelper(lgrassdPath, tabId, alias, event.sender)
      const remoteName = posix.basename(absoluteRemote(remote))
      const fileName = dir ? `${remoteName}.tar.gz` : remoteName
      const win = BrowserWindow.fromWebContents(event.sender)
      const options = {
        title: `Download ${fileName}`,
        defaultPath: join(app.getPath('downloads'), fileName)
      }
      const picked = win
        ? await dialog.showSaveDialog(win, options)
        : await dialog.showSaveDialog(options)
      if (picked.canceled || !picked.filePath) return { ok: true, result: null }
      const body = {
        op: dir ? 'downloadDir' : 'download',
        remote,
        local: picked.filePath,
        replace: true
      }
      return startTransfer(helper, body, randomUUID(), basename(picked.filePath))
    }
  )

  ipcMain.handle('ssh:sessionPickUploads', async (event): Promise<string[]> => {
    const win = BrowserWindow.fromWebContents(event.sender)
    const options = {
      title: 'Upload files',
      properties: ['openFile' as const, 'multiSelections' as const]
    }
    const picked = win
      ? await dialog.showOpenDialog(win, options)
      : await dialog.showOpenDialog(options)
    return picked.canceled ? [] : picked.filePaths
  })

  // A copy between two server tabs, staged locally. Both tabs live in the window that asks, so its
  // sender serves either side's helper and receives the destination's events.
  ipcMain.handle('ssh:sessionRelay', (event, req: SSHRelayRequest) => {
    if (req.fromAlias === req.toAlias)
      throw new Error('ssh: a copy within one server is a copy op.')
    absoluteRemote(req.remote)
    absoluteRemote(req.remoteDir)
    relaySenders.set(req.fromTab, event.sender)
    relaySenders.set(req.toTab, event.sender)
    watchSender(event.sender)
    return hub.start({ ...req, size: Number(req.size) || 0, dir: req.dir === true })
  })
  ipcMain.handle('ssh:sessionRelayCancel', (_event, id: string) => hub.cancel(id))

  ipcMain.on('ssh:sessionStop', (_event, tabId: string) => stopSession(tabId))
  ipcMain.handle('ssh:sessionSetSudo', (_event, tabId: string, on: boolean) =>
    setSudo(tabId, on === true)
  )
  // Staging goes first: ending the sessions ends their relays, which forgets their folders.
  app.on('will-quit', () => {
    removeStagesNow(hub.stages())
    stopAllSessions()
  })
}
