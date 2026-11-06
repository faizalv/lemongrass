// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createSessionLink, type SessionMessage } from './sshSessionLink.ts'

function setup(): {
  sent: SessionMessage[]
  events: SessionMessage[]
  link: ReturnType<typeof createSessionLink>
} {
  const sent: SessionMessage[] = []
  const events: SessionMessage[] = []
  const link = createSessionLink(
    (line) => {
      assert.ok(line.endsWith('\n'))
      sent.push(JSON.parse(line) as SessionMessage)
    },
    (event) => events.push(event)
  )
  return { sent, events, link }
}

test('a reply resolves its request by id, out of order', async () => {
  const { sent, link } = setup()
  const home = link.request({ op: 'home' })
  const list = link.request({ op: 'list', path: '/srv' })
  assert.deepEqual(sent, [
    { op: 'home', id: '1' },
    { op: 'list', path: '/srv', id: '2' }
  ])
  link.feed('{"id":"2","ok":true,"result":{"path":"/srv","entries":[]}}\n')
  link.feed('{"id":"1","ok":true,"result":{"path":"/home/u"}}\n')
  assert.deepEqual(await list, { ok: true, result: { path: '/srv', entries: [] } })
  assert.deepEqual(await home, { ok: true, result: { path: '/home/u' } })
})

test('an error reply keeps its code', async () => {
  const { link } = setup()
  const reply = link.request({ op: 'home' })
  link.feed('{"id":"1","ok":false,"error":"no master","code":"notLoggedIn"}\n')
  assert.deepEqual(await reply, { ok: false, error: 'no master', code: 'notLoggedIn' })
})

test('lines split across chunks and events are routed apart from replies', async () => {
  const { events, link } = setup()
  const reply = link.request({ op: 'mkdir', path: '/a' })
  link.feed('{"event":"progress","transfer":"t","do')
  link.feed('ne":5,"total":10}\n{"id":"1","ok":true}\n{"event":"done",')
  link.feed('"transfer":"t"}\nnot json\n')
  assert.deepEqual(await reply, { ok: true, result: {} })
  assert.deepEqual(events, [
    { event: 'progress', transfer: 't', done: 5, total: 10 },
    { event: 'done', transfer: 't' }
  ])
})

test('close answers every waiting request with the error', async () => {
  const { link } = setup()
  const a = link.request({ op: 'home' })
  const b = link.request({ op: 'list', path: '/' })
  link.close('stopped')
  assert.deepEqual(await a, { ok: false, error: 'stopped' })
  assert.deepEqual(await b, { ok: false, error: 'stopped' })
})

test('a line far larger than one chunk arrives whole', async () => {
  const { link } = setup()
  const reply = link.request({ op: 'read', path: '/big' })
  const content = 'x'.repeat(3_000_000)
  const line = `${JSON.stringify({ id: '1', ok: true, result: { content } })}\n`
  for (let at = 0; at < line.length; at += 65_536) link.feed(line.slice(at, at + 65_536))
  const got = await reply
  assert.ok(got.ok)
  assert.equal((got.result as { content: string }).content.length, content.length)
})
