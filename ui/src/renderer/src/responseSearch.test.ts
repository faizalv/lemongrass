// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { parseQuery, pathId, runSearch, searchJson, type SearchQuery } from './responseSearch.ts'

const body = {
  data: {
    items: [
      { id: 1, status: 'paid', price: 150, site: 'shop.example.test' },
      { id: 2, status: 'Pending', price: 80, note: null },
      { id: 3, status: 'paid', price: 100, tags: ['Paid', 'gift'] }
    ],
    total: 3
  }
}

function paths(input: string): string[] {
  return searchJson(body, parseQuery(input) as SearchQuery).matches.map((m) => m.path.join('.'))
}

test('the query kind follows the order compare, key:value, path, text', () => {
  assert.equal(parseQuery('price>=100')?.kind, 'compare')
  assert.equal(parseQuery('status:paid')?.kind, 'keyValue')
  assert.equal(parseQuery('data.items.*.id')?.kind, 'path')
  assert.equal(parseQuery('paid order')?.kind, 'text')
  assert.equal(parseQuery('"price>100"')?.kind, 'text')
  assert.equal(parseQuery('   '), null)
})

test('plain text matches keys and values, ignoring case, in document order', () => {
  assert.deepEqual(paths('paid'), [
    'data.items.0.status',
    'data.items.2.status',
    'data.items.2.tags.0'
  ])
  const [status] = searchJson(body, parseQuery('STATUS') as SearchQuery).matches
  assert.deepEqual([status.key, status.value], [true, false])
})

test('key:value matches a value under a key name at any depth', () => {
  assert.deepEqual(paths('status:pend'), ['data.items.1.status'])
  assert.deepEqual(paths('data.items.*.status:paid'), [
    'data.items.0.status',
    'data.items.2.status'
  ])
  assert.deepEqual(paths('items:'), ['data.items'])
  assert.deepEqual(paths('tags:gift'), ['data.items.2.tags.1'])
})

test('a path picks fields with * for any key or index, and falls back to text', () => {
  assert.deepEqual(paths('data.items.*.id'), [
    'data.items.0.id',
    'data.items.1.id',
    'data.items.2.id'
  ])
  assert.deepEqual(paths('data.items.1.price'), ['data.items.1.price'])
  assert.deepEqual(paths('shop.example'), ['data.items.0.site'])
})

test('comparisons are numeric when both sides are numbers, else equality ignores case', () => {
  assert.deepEqual(paths('price>100'), ['data.items.0.price'])
  assert.deepEqual(paths('price<=100'), ['data.items.1.price', 'data.items.2.price'])
  assert.deepEqual(paths('status=paid'), ['data.items.0.status', 'data.items.2.status'])
  assert.deepEqual(paths('status!=paid'), ['data.items.1.status'])
  assert.deepEqual(paths('note=null'), ['data.items.1.note'])
  assert.deepEqual(paths('status>1'), [])
})

test('ancestors cover every level above a match', () => {
  const result = searchJson(body, parseQuery('gift') as SearchQuery)
  for (const path of [
    [],
    ['data'],
    ['data', 'items'],
    ['data', 'items', 2],
    ['data', 'items', 2, 'tags']
  ]) {
    assert.ok(result.ancestors.has(pathId(path)), pathId(path))
  }
  assert.ok(!result.ancestors.has(pathId(['data', 'items', 0])))
})

function records(input: string, root: unknown = body): string[] {
  const outcome = runSearch(root, input)
  assert.ok(outcome && 'result' in outcome, input)
  assert.equal(outcome.result.unit, 'record')
  return outcome.result.steps.map((p) => p.join('.'))
}

const orders = {
  orders: [
    {
      customer: 'ann',
      lines: [
        { sku: 'X', qty: 1 },
        { sku: 'Y', qty: 5 }
      ]
    },
    { customer: 'bob', lines: [{ sku: 'X', qty: 3 }] }
  ],
  total: 2
}

test('AND pairs fields only inside one record', () => {
  assert.deepEqual(records('status:paid AND price>100'), ['data.items.0'])
  assert.deepEqual(records('status:paid AND price>200'), [])
  assert.deepEqual(records('sku:X AND qty>2', orders), ['orders.1.lines.0'])
  assert.deepEqual(records('customer:ann AND sku:Y', orders), ['orders.0'])
})

test('OR, NOT and parentheses', () => {
  assert.deepEqual(records('status=pending OR price>120'), ['data.items.0', 'data.items.1'])
  assert.deepEqual(records('NOT status:paid'), ['data.items.1'])
  assert.deepEqual(records('(status=paid OR status=pending) AND NOT price>=100'), ['data.items.1'])
  assert.deepEqual(records('price>=100 and not tags:gift'), ['data.items.0'])
})

test('a term outside every list is judged on the root', () => {
  assert.deepEqual(records('total=2 AND customer:bob', orders), [''])
  assert.deepEqual(records('total=3 AND customer:bob', orders), [])
})

test('highlights are the matches of terms not under NOT inside qualifying records', () => {
  const outcome = runSearch(body, 'status:paid AND NOT tags:gift')
  assert.ok(outcome && 'result' in outcome)
  assert.deepEqual(
    outcome.result.matches.map((m) => m.path.join('.')),
    ['data.items.0.status']
  )
  assert.ok(outcome.result.records.has(pathId(['data', 'items', 0])))
  assert.ok(outcome.result.ancestors.has(pathId(['data', 'items'])))
})

test('words between operators form one term, quotes keep operators as text, and errors are reported', () => {
  const single = runSearch(body, 'paid')
  assert.ok(single && 'result' in single && single.result.unit === 'match')
  const quoted = runSearch({ a: 'rock AND roll' }, '"rock AND roll"')
  assert.ok(quoted && 'result' in quoted && quoted.result.unit === 'match')
  assert.equal(quoted.result.matches.length, 1)
  assert.deepEqual(runSearch(body, 'status:paid AND'), { error: 'Invalid query' })
  assert.deepEqual(runSearch(body, '(status:paid'), { error: 'Invalid query' })
  assert.equal(runSearch(body, '  '), null)
})
