// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { SSHSessionCode, SSHSessionReply } from '../preload/types'

// The file helper's wire, one JSON object per line each way: a request carries a string id and
// gets one reply under it, and a message with an `event` field arrives unprompted. Kept free of
// processes and sibling imports so it runs under the plain node test runner.

export type SessionMessage = Record<string, unknown>

export interface SessionLink {
  request(body: SessionMessage): Promise<SSHSessionReply>
  feed(chunk: string): void
  // Every request still waiting gets the error as its reply, so no caller hangs on a dead helper.
  close(error: string): void
}

export function createSessionLink(
  write: (line: string) => void,
  onEvent: (event: SessionMessage) => void
): SessionLink {
  let next = 0
  // The unfinished line's chunks, joined only once its newline arrives. A 2 MB file's read
  // reply is one line of up to about 12 MB, and rejoining a growing string per chunk would be
  // quadratic.
  let partial: string[] = []
  const pending = new Map<string, (reply: SSHSessionReply) => void>()

  const handle = (line: string): void => {
    let msg: SessionMessage
    try {
      msg = JSON.parse(line) as SessionMessage
    } catch {
      return
    }
    if (typeof msg.event === 'string') {
      onEvent(msg)
      return
    }
    const id = String(msg.id)
    const resolve = pending.get(id)
    if (!resolve) return
    pending.delete(id)
    resolve(
      msg.ok === true
        ? { ok: true, result: msg.result ?? {} }
        : {
            ok: false,
            error: typeof msg.error === 'string' ? msg.error : 'The file helper failed.',
            code: typeof msg.code === 'string' ? (msg.code as SSHSessionCode) : undefined
          }
    )
  }

  return {
    request(body) {
      const id = String(++next)
      return new Promise((resolve) => {
        pending.set(id, resolve)
        write(`${JSON.stringify({ ...body, id })}\n`)
      })
    },
    feed(chunk) {
      let start = 0
      let end = chunk.indexOf('\n')
      while (end !== -1) {
        partial.push(chunk.slice(start, end))
        const line = partial.join('')
        partial = []
        if (line.trim()) handle(line)
        start = end + 1
        end = chunk.indexOf('\n', start)
      }
      if (start < chunk.length) partial.push(chunk.slice(start))
    },
    close(error) {
      for (const resolve of pending.values()) resolve({ ok: false, error })
      pending.clear()
    }
  }
}
