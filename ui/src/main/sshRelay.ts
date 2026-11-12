// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { randomUUID } from 'node:crypto'
import { posix } from 'node:path'
import type {
  SSHRelayRequest,
  SSHSessionEvent,
  SSHSessionReply,
  SSHTransferStart
} from '../preload/types'

// Copies a file or folder from one server tab to another through a local staging folder: the
// source tab's helper downloads into it, then the destination tab's helper uploads from it. Each
// side keeps its own login and sudo mode, and the destination tab sees one transfer under the
// relay's own id. Kept free of Electron so it runs under the plain node test runner.

type HelperBody = Record<string, unknown>
type Outgoing = Exclude<SSHSessionEvent, { event: 'exit' }>

export interface RelayPorts {
  // Starts a request on a tab's helper, starting the helper when it is not running.
  request(tabId: string, alias: string, body: HelperBody): Promise<SSHSessionReply>
  // Sends an event to the destination tab, as if its own helper had sent it.
  emit(event: Outgoing): void
  // A new empty staging folder, and whatever filesystem room it has.
  stage(): Promise<string>
  freeBytes(dir: string): Promise<number>
  removeStage(dir: string): Promise<void>
  join(dir: string, name: string): string
}

interface Relay {
  id: string
  req: SSHRelayRequest
  name: string
  stageDir: string
  local: string
  phase: 'download' | 'upload'
  // The helper transfer id of the running phase, and the tab whose helper runs it.
  helperId: string
  helperTab: string
  helperAlias: string
  total: number
  cancelled: boolean
}

export interface RelayHub {
  start(req: SSHRelayRequest): Promise<SSHSessionReply<SSHTransferStart>>
  cancel(id: string): Promise<void>
  // A helper event; true when it belongs to a relay and must not reach the tab as it is.
  claim(tabId: string, event: HelperBody): boolean
  // A helper process ended, so whatever phase it ran is over.
  helperEnded(tabId: string, error: string): void
  // A server tab closed: every relay from or to it ends, so none starts a helper for a gone tab.
  tabClosed(tabId: string): void
  // Staging folders still in use, for removal on quit.
  stages(): string[]
}

const CODE_TEXT: Record<string, (alias: string) => string> = {
  notLoggedIn: (alias) => `Log in in the ${alias} terminal first.`,
  exists: () => 'That name already exists there.',
  sudoPassword: (alias) => `sudo on ${alias} asks for a password.`
}

function failText(alias: string, reply: { error: string; code?: string }): string {
  const known = reply.code ? CODE_TEXT[reply.code] : undefined
  if (known) return known(alias)
  return `${alias}: ${reply.error.replace(/^sshsession: /, '')}`
}

// A file's bar fills in two halves, download then upload; a folder's archive size is unknown
// until it is staged, so its download shows bytes only and its upload fills the bar.
export function relayProgress(
  phase: Relay['phase'],
  dir: boolean,
  size: number,
  done: number,
  total: number
): { done: number; total: number } {
  if (dir) return phase === 'download' ? { done, total: 0 } : { done, total }
  return phase === 'download' ? { done, total: size * 2 } : { done: size + done, total: size * 2 }
}

export function createRelayHub(ports: RelayPorts): RelayHub {
  const relays = new Map<string, Relay>()
  // Helper transfers of relays that already ended, such as one whose tab closed, still send their
  // last events; they are swallowed until the helper's own end, so no tab shows a stray row.
  const retired = new Map<string, string>()

  // A helper that cannot start throws, even synchronously; the relay treats that as the phase
  // failing.
  const ask = async (tabId: string, alias: string, body: HelperBody): Promise<SSHSessionReply> => {
    try {
      return await ports.request(tabId, alias, body)
    } catch (err) {
      return { ok: false, error: err instanceof Error ? err.message : String(err) }
    }
  }

  const byHelper = (helperId: string): Relay | undefined => {
    for (const r of relays.values()) if (r.helperId === helperId) return r
    return undefined
  }

  const end = async (r: Relay, error?: string, code?: 'cancelled'): Promise<void> => {
    if (!relays.delete(r.id)) return
    const to = r.req.toTab
    if (error === undefined) ports.emit({ tabId: to, event: 'done', transfer: r.id })
    else ports.emit({ tabId: to, event: 'failed', transfer: r.id, error, code })
    await ports.removeStage(r.stageDir).catch(() => undefined)
  }

  const fail = (r: Relay, error: string): Promise<void> =>
    r.cancelled ? end(r, 'Cancelled.', 'cancelled') : end(r, error)

  const upload = async (r: Relay): Promise<void> => {
    r.phase = 'upload'
    r.helperId = randomUUID()
    r.helperTab = r.req.toTab
    r.helperAlias = r.req.toAlias
    const body = {
      op: r.req.dir ? 'uploadDir' : 'upload',
      local: r.local,
      remote: posix.join(r.req.remoteDir, r.name),
      transfer: r.helperId
    }
    const reply = await ask(r.req.toTab, r.req.toAlias, body)
    if (!reply.ok) return fail(r, failText(r.req.toAlias, reply))
    // Cancel may have landed while the upload was starting, before its id could be cancelled.
    if (r.cancelled) await cancelPhase(r)
  }

  const cancelPhase = async (r: Relay): Promise<void> => {
    await ask(r.helperTab, r.helperAlias, { op: 'cancel', transfer: r.helperId })
  }

  return {
    async start(req) {
      const name = posix.basename(req.remote)
      if (!name || name === '/') return { ok: false, error: 'Pick a file or folder to copy.' }
      const stageDir = await ports.stage()
      if (!req.dir) {
        const free = await ports.freeBytes(stageDir).catch(() => Infinity)
        if (free < req.size) {
          await ports.removeStage(stageDir).catch(() => undefined)
          return {
            ok: false,
            error: `Not enough room to stage ${name} locally: it needs ${req.size} bytes and ${free} are free.`
          }
        }
      }
      const r: Relay = {
        id: randomUUID(),
        req,
        name,
        stageDir,
        local: ports.join(stageDir, req.dir ? `${name}.tar.gz` : name),
        phase: 'download',
        helperId: randomUUID(),
        helperTab: req.fromTab,
        helperAlias: req.fromAlias,
        total: req.dir ? 0 : req.size * 2,
        cancelled: false
      }
      relays.set(r.id, r)
      const body = {
        op: req.dir ? 'downloadDir' : 'download',
        remote: req.remote,
        local: r.local,
        transfer: r.helperId
      }
      const reply = await ask(req.fromTab, req.fromAlias, body)
      if (!reply.ok) {
        relays.delete(r.id)
        await ports.removeStage(stageDir).catch(() => undefined)
        return { ok: false, error: failText(req.fromAlias, reply) }
      }
      return { ok: true, result: { transfer: r.id, name, total: r.total } }
    },

    async cancel(id) {
      const r = relays.get(id)
      if (!r || r.cancelled) return
      r.cancelled = true
      await cancelPhase(r)
    },

    claim(tabId, event) {
      if (typeof event.transfer === 'string' && retired.has(event.transfer)) {
        if (event.event !== 'progress') retired.delete(event.transfer)
        return true
      }
      const r = typeof event.transfer === 'string' ? byHelper(event.transfer) : undefined
      if (!r || r.helperTab !== tabId) return false
      if (event.event === 'progress') {
        const p = relayProgress(
          r.phase,
          r.req.dir,
          r.req.size,
          Number(event.done) || 0,
          Number(event.total) || 0
        )
        ports.emit({ tabId: r.req.toTab, event: 'progress', transfer: r.id, ...p })
      } else if (event.event === 'done') {
        if (r.phase === 'download' && !r.cancelled) void upload(r)
        else if (r.phase === 'download') void fail(r, 'Cancelled.')
        else void end(r)
      } else if (event.event === 'failed') {
        const code = event.code === 'cancelled' ? 'cancelled' : undefined
        void fail(r, code ? 'Cancelled.' : failText(r.helperAlias, { error: String(event.error) }))
      }
      return true
    },

    helperEnded(tabId, error) {
      for (const [id, tab] of retired) if (tab === tabId) retired.delete(id)
      for (const r of [...relays.values()]) {
        if (r.helperTab === tabId) void fail(r, failText(r.helperAlias, { error }))
      }
    },

    tabClosed(tabId) {
      for (const r of [...relays.values()]) {
        if (r.req.fromTab !== tabId && r.req.toTab !== tabId) continue
        r.cancelled = true
        retired.set(r.helperId, r.helperTab)
        void end(r, 'Cancelled.', 'cancelled')
        if (r.helperTab !== tabId) void cancelPhase(r)
      }
    },

    stages() {
      return [...relays.values()].map((r) => r.stageDir)
    }
  }
}
