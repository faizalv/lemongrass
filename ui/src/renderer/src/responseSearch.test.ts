// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { parseQuery, pathId, searchJson, type SearchQuery } from './responseSearch.ts'

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
