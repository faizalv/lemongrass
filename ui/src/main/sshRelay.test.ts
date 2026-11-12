// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createRelayHub, relayProgress, type RelayPorts } from './sshRelay.ts'
import type { SSHRelayRequest, SSHSessionEvent, SSHSessionReply } from '../preload/types'

type Body = Record<string, unknown>

interface Setup {
  hub: ReturnType<typeof createRelayHub>
  requests: { tabId: string; alias: string; body: Body }[]
  emitted: SSHSessionEvent[]
  removed: string[]
}

function setup(opts: { free?: number; reply?: (body: Body) => SSHSessionReply } = {}): Setup {
  const requests: { tabId: string; alias: string; body: Body }[] = []
  const emitted: SSHSessionEvent[] = []
  const removed: string[] = []
  let stages = 0
  const ports: RelayPorts = {
    // Not async, so a throwing reply throws synchronously like a helper that cannot start.
    request: (tabId, alias, body) => {
      requests.push({ tabId, alias, body })
      const reply = opts.reply?.(body)
      return Promise.resolve(reply ?? { ok: true, result: { transfer: body.transfer, total: 10 } })
    },
    emit: (event) => emitted.push(event),
    stage: async () => `/tmp/stage-${++stages}`,
    freeBytes: async () => opts.free ?? Infinity,
    removeStage: async (dir) => {
      removed.push(dir)
    },
    join: (dir, name) => `${dir}/${name}`
  }
  return { hub: createRelayHub(ports), requests, emitted, removed }
}

const settle = (): Promise<void> => new Promise((resolve) => setImmediate(resolve))

const fileReq: SSHRelayRequest = {
  fromTab: 'a',
  fromAlias: 'prod',
  remote: '/var/log/app.log',
  dir: false,
  size: 100,
  toTab: 'b',
  toAlias: 'staging',
  remoteDir: '/srv'
}

test('a file downloads into staging, uploads from it, then the staging is removed', async () => {
  const { hub, requests, emitted, removed } = setup()
  const reply = await hub.start(fileReq)
  assert.ok(reply.ok)
  const id = reply.result.transfer
  assert.deepEqual(reply.result, { transfer: id, name: 'app.log', total: 200 })
  const down = requests[0]
  assert.equal(down.tabId, 'a')
  assert.deepEqual(
    { ...down.body, transfer: undefined },
    {
      op: 'download',
      remote: '/var/log/app.log',
      local: '/tmp/stage-1/app.log',
      transfer: undefined
    }
  )

  assert.equal(
    hub.claim('a', { event: 'progress', transfer: down.body.transfer, done: 40, total: 100 }),
    true
  )
  assert.equal(hub.claim('a', { event: 'done', transfer: down.body.transfer }), true)
  await settle()
  const up = requests[1]
  assert.equal(up.tabId, 'b')
  assert.equal(up.body.op, 'upload')
  assert.equal(up.body.local, '/tmp/stage-1/app.log')
  assert.equal(up.body.remote, '/srv/app.log')

  hub.claim('b', { event: 'progress', transfer: up.body.transfer, done: 30, total: 100 })
  hub.claim('b', { event: 'done', transfer: up.body.transfer })
  await settle()
  assert.deepEqual(emitted, [
    { tabId: 'b', event: 'progress', transfer: id, done: 40, total: 200 },
    { tabId: 'b', event: 'progress', transfer: id, done: 130, total: 200 },
    { tabId: 'b', event: 'done', transfer: id }
  ])
  assert.deepEqual(removed, ['/tmp/stage-1'])
  assert.deepEqual(hub.stages(), [])
})

test('a folder travels as an archive and unpacks with uploadDir', async () => {
  const { hub, requests } = setup()
  const reply = await hub.start({ ...fileReq, remote: '/srv/logs', dir: true, size: 0 })
  assert.ok(reply.ok)
  assert.equal(reply.result.total, 0)
  assert.equal(requests[0].body.op, 'downloadDir')
  assert.equal(requests[0].body.local, '/tmp/stage-1/logs.tar.gz')
  hub.claim('a', { event: 'done', transfer: requests[0].body.transfer })
  await settle()
  assert.equal(requests[1].body.op, 'uploadDir')
  assert.equal(requests[1].body.remote, '/srv/logs')
})

test('progress maps a file into two halves and a folder into bytes then its archive', () => {
  assert.deepEqual(relayProgress('download', false, 100, 50, 100), { done: 50, total: 200 })
  assert.deepEqual(relayProgress('upload', false, 100, 50, 100), { done: 150, total: 200 })
  assert.deepEqual(relayProgress('download', true, 0, 500, 0), { done: 500, total: 0 })
  assert.deepEqual(relayProgress('upload', true, 0, 50, 80), { done: 50, total: 80 })
})

test('a file larger than the staging room is refused before anything starts', async () => {
  const { hub, requests, removed } = setup({ free: 50 })
  const reply = await hub.start(fileReq)
  assert.equal(reply.ok, false)
  assert.match(!reply.ok ? reply.error : '', /Not enough room/)
  assert.equal(requests.length, 0)
  assert.deepEqual(removed, ['/tmp/stage-1'])
})

test('a refused download names the source server and removes the staging', async () => {
  const { hub, removed } = setup({ reply: () => ({ ok: false, error: 'x', code: 'notLoggedIn' }) })
  const reply = await hub.start(fileReq)
  assert.deepEqual(reply, { ok: false, error: 'Log in in the prod terminal first.' })
  assert.deepEqual(removed, ['/tmp/stage-1'])
})

test('a failed upload ends the relay with the destination named', async () => {
  const { hub, requests, emitted, removed } = setup({
    reply: (body) =>
      body.op === 'upload'
        ? { ok: false, error: 'sshsession: x', code: 'exists' }
        : { ok: true, result: {} }
  })
  const reply = await hub.start(fileReq)
  assert.ok(reply.ok)
  hub.claim('a', { event: 'done', transfer: requests[0].body.transfer })
  await settle()
  await settle()
  assert.deepEqual(emitted.at(-1), {
    tabId: 'b',
    event: 'failed',
    transfer: reply.result.transfer,
    error: 'That name already exists there.',
    code: undefined
  })
  assert.deepEqual(removed, ['/tmp/stage-1'])
})

test('cancel reaches the running phase and ends as cancelled', async () => {
  const { hub, requests, emitted } = setup()
  const reply = await hub.start(fileReq)
  assert.ok(reply.ok)
  const downId = requests[0].body.transfer
  await hub.cancel(reply.result.transfer)
  assert.deepEqual(requests[1], {
    tabId: 'a',
    alias: 'prod',
    body: { op: 'cancel', transfer: downId }
  })
  hub.claim('a', { event: 'failed', transfer: downId, error: 'cancelled', code: 'cancelled' })
  await settle()
  const last = emitted.at(-1)
  assert.equal(last?.event, 'failed')
  assert.equal(last?.event === 'failed' && last.code, 'cancelled')
})

test('a closed tab ends its relays and its helper events are swallowed', async () => {
  const { hub, requests, emitted, removed } = setup()
  const reply = await hub.start(fileReq)
  assert.ok(reply.ok)
  const downId = requests[0].body.transfer
  hub.tabClosed('b')
  await settle()
  assert.equal(emitted.at(-1)?.event, 'failed')
  assert.deepEqual(requests[1].body, { op: 'cancel', transfer: downId })
  assert.deepEqual(removed, ['/tmp/stage-1'])
  assert.equal(hub.claim('a', { event: 'progress', transfer: downId, done: 1, total: 100 }), true)
  assert.equal(hub.claim('a', { event: 'failed', transfer: downId, error: 'c' }), true)
  assert.equal(hub.claim('a', { event: 'failed', transfer: downId, error: 'c' }), false)
})

test('a helper that dies fails the relay whose phase it ran', async () => {
  const { hub, emitted } = setup()
  const reply = await hub.start(fileReq)
  assert.ok(reply.ok)
  hub.helperEnded('a', 'ssh: the file helper stopped (exit 1).')
  await settle()
  assert.deepEqual(emitted.at(-1), {
    tabId: 'b',
    event: 'failed',
    transfer: reply.result.transfer,
    error: 'prod: ssh: the file helper stopped (exit 1).',
    code: undefined
  })
})

test('a helper that throws on start fails the relay instead of escaping', async () => {
  const { hub, removed } = setup({
    reply: () => {
      throw new Error('ssh: the server tab is closed.')
    }
  })
  const reply = await hub.start(fileReq)
  assert.deepEqual(reply, { ok: false, error: 'prod: ssh: the server tab is closed.' })
  assert.deepEqual(removed, ['/tmp/stage-1'])
})

test('events of a helper transfer the relay does not own pass through', () => {
  const { hub } = setup()
  assert.equal(hub.claim('a', { event: 'done', transfer: 'other' }), false)
})
