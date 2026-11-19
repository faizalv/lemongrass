// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  dropResponseState,
  responseViewOf,
  setResponseView,
  setTreeToggle,
  treeToggleOf
} from './resterTreeState.ts'

test('a node never toggled has no saved choice', () => {
  assert.equal(treeToggleOf('req-a', '["a"]'), undefined)
})

test('a closed node is remembered as closed, not as untouched', () => {
  setTreeToggle('req-a', '["a"]', false)
  assert.equal(treeToggleOf('req-a', '["a"]'), false)
  setTreeToggle('req-a', '["a"]', true)
  assert.equal(treeToggleOf('req-a', '["a"]'), true)
})

test('choices are kept per request and dropped with it', () => {
  setTreeToggle('req-b', '[]', true)
  setTreeToggle('req-c', '[]', false)
  dropResponseState('req-b')
  assert.equal(treeToggleOf('req-b', '[]'), undefined)
  assert.equal(treeToggleOf('req-c', '[]'), false)
})

test('a request with no saved view shows the tree with no search', () => {
  assert.deepEqual(responseViewOf('req-new'), { view: 'tree', query: '', filter: false })
})

test('a patch changes only the fields it names, per request', () => {
  setResponseView('req-d', { view: 'raw' })
  setResponseView('req-d', { query: 'sku' })
  assert.deepEqual(responseViewOf('req-d'), { view: 'raw', query: 'sku', filter: false })
  assert.equal(responseViewOf('req-e').view, 'tree')
})

test('dropping a request forgets its view too', () => {
  setResponseView('req-f', { filter: true })
  dropResponseState('req-f')
  assert.equal(responseViewOf('req-f').filter, false)
})
