// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { dropTreeToggles, setTreeToggle, treeToggleOf } from './resterTreeState.ts'

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
  dropTreeToggles('req-b')
  assert.equal(treeToggleOf('req-b', '[]'), undefined)
  assert.equal(treeToggleOf('req-c', '[]'), false)
})
