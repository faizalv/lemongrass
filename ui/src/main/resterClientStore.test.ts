// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// @ts-ignore node:assert has no types in the node tsconfig
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { existsSync, mkdirSync, mkdtempSync, writeFileSync } from 'fs'
import { tmpdir } from 'os'
import { join } from 'path'
import {
  appendRun,
  clearHistory,
  deleteRun,
  loadHistory,
  loadRequests,
  loadRun
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

const MB = 1 << 20

function ids(runs: { id: string }[]): string[] {
  return runs.map((r) => r.id)
}

test('history keeps the newest runs first, up to the run limit', () => {
  const dir = tempDir()
  for (let i = 0; i < 15; i++) appendRun(dir, 'req-1', run(`r${i}`), 10, 10 * MB)
  const history = loadHistory(dir, 'req-1')
  assert.equal(history.length, 10)
  assert.equal(history[0].id, 'r14')
  assert.equal(loadRun(dir, 'req-1', 'r4'), null)
})

test('a large body is kept whole and listed without its body', () => {
  const dir = tempDir()
  const body = JSON.stringify({ items: 'x'.repeat(3 * MB) })
  const [summary] = appendRun(dir, 'req-1', run('big', body), 10, 10 * MB)
  assert.equal('body' in summary.result, false)
  assert.equal(summary.bodyCut, false)
  assert.equal(loadRun(dir, 'req-1', 'big').result.body, body)
})

test('older runs are kept only while the size budget holds', () => {
  const dir = tempDir()
  for (let i = 0; i < 5; i++) appendRun(dir, 'req-1', run(`r${i}`, 'x'.repeat(3 * MB)), 10, 10 * MB)
  assert.deepEqual(ids(loadHistory(dir, 'req-1')), ['r4', 'r3', 'r2'])
  assert.equal(loadRun(dir, 'req-1', 'r1'), null)
})

test('the newest run is kept even when it alone passes the budget', () => {
  const dir = tempDir()
  appendRun(dir, 'req-1', run('small'), 10, 1 * MB)
  appendRun(dir, 'req-1', run('huge', 'x'.repeat(2 * MB)), 10, 1 * MB)
  assert.deepEqual(ids(loadHistory(dir, 'req-1')), ['huge'])
  assert.equal(loadRun(dir, 'req-1', 'small'), null)
})

test('a single history file is moved into per-run files once', () => {
  const dir = tempDir()
  mkdirSync(join(dir, 'history'), { recursive: true })
  const legacy = join(dir, 'history', 'req-1.json')
  writeFileSync(legacy, JSON.stringify([run('b', '{"b":1}'), run('a')]))
  assert.deepEqual(ids(loadHistory(dir, 'req-1')), ['b', 'a'])
  assert.equal(existsSync(legacy), false)
  assert.equal(loadRun(dir, 'req-1', 'b').result.body, '{"b":1}')
})

test('runs can be deleted and the history cleared', () => {
  const dir = tempDir()
  appendRun(dir, 'req-1', run('a'), 10, 10 * MB)
  appendRun(dir, 'req-1', run('b'), 10, 10 * MB)
  assert.deepEqual(ids(deleteRun(dir, 'req-1', 'a')), ['b'])
  assert.equal(loadRun(dir, 'req-1', 'a'), null)
  clearHistory(dir, 'req-1')
  assert.deepEqual(loadHistory(dir, 'req-1'), [])
})

test('a request or run id that is not a plain id is refused', () => {
  assert.throws(() => loadHistory(tempDir(), '../escape'))
  assert.throws(() => loadRun(tempDir(), 'req-1', '../escape'))
})
