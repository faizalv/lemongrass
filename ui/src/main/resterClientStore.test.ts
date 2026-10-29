// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

// @ts-ignore node:assert has no types in the node tsconfig
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { existsSync, mkdtempSync, writeFileSync } from 'fs'
import { tmpdir } from 'os'
import { join } from 'path'
import {
  HISTORY_BODY_BYTES,
  HISTORY_LIMIT,
  appendRun,
  clearHistory,
  deleteRun,
  loadHistory,
  loadRequests
  // @ts-ignore node --test imports the .ts source by its extension
} from './resterClientStore.ts'

function tempDir(): string {
  return mkdtempSync(join(tmpdir(), 'rester-client-'))
}

function request(id: string): Record<string, unknown> {
  return {
    id,
    name: id,
    rester: 'shop',
    environment: 'staging',
    user: 'alice',
    method: 'GET',
    url: '/a',
    headers: [],
    bodyKind: 'none',
    json: '',
    form: []
  }
}

function run(id: string, body = 'ok'): Record<string, unknown> {
  return {
    id,
    at: '2026-10-29T00:00:00Z',
    request: {},
    result: {
      status: 200,
      headers: {},
      body,
      binary: false,
      size: body.length,
      truncated: false,
      durationMs: 1,
      url: ''
    },
    download: null,
    error: '',
    bodyCut: false
  }
}

test('per-project lists are merged once into the global list', () => {
  const dir = tempDir()
  const legacy = join(dir, 'legacy.json')
  writeFileSync(
    legacy,
    JSON.stringify({ p1: [request('a'), request('b')], p2: [request('b'), request('c')] })
  )
  const store = join(dir, 'store')
  assert.deepEqual(
    loadRequests(store, legacy).map((r: { id: string }) => r.id),
    ['a', 'b', 'c']
  )
  assert.equal(existsSync(legacy), false)
  assert.equal(loadRequests(store, legacy).length, 3)
})

test('history keeps the newest runs first, up to the limit', () => {
  const dir = tempDir()
  for (let i = 0; i < HISTORY_LIMIT + 5; i++) appendRun(dir, 'req-1', run(`r${i}`))
  const history = loadHistory(dir, 'req-1')
  assert.equal(history.length, HISTORY_LIMIT)
  assert.equal(history[0].id, `r${HISTORY_LIMIT + 4}`)
})

test('a long response body is cut and marked', () => {
  const dir = tempDir()
  const [stored] = appendRun(dir, 'req-1', run('big', 'x'.repeat(HISTORY_BODY_BYTES + 10)))
  assert.equal(stored.result.body.length, HISTORY_BODY_BYTES)
  assert.equal(stored.bodyCut, true)
})

test('runs can be deleted and the history cleared', () => {
  const dir = tempDir()
  appendRun(dir, 'req-1', run('a'))
  appendRun(dir, 'req-1', run('b'))
  assert.deepEqual(
    deleteRun(dir, 'req-1', 'a').map((r: { id: string }) => r.id),
    ['b']
  )
  clearHistory(dir, 'req-1')
  assert.deepEqual(loadHistory(dir, 'req-1'), [])
})

test('a request id that is not a plain id is refused', () => {
  assert.throws(() => loadHistory(tempDir(), '../escape'))
})
