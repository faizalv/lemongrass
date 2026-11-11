// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { monogram } from './monogram.ts'

test('two words give their first letters', () => {
  assert.equal(monogram('kencana-backend'), 'KB')
  assert.equal(monogram('order_service'), 'OS')
  assert.equal(monogram('Acme Portal web'), 'AP')
  assert.equal(monogram('billingApi'), 'BA')
})

test('a single word gives its first two letters', () => {
  assert.equal(monogram('lemongrass'), 'LE')
  assert.equal(monogram('x'), 'X')
})

test('separators alone fall back to a placeholder', () => {
  assert.equal(monogram(''), '?')
  assert.equal(monogram('--'), '?')
})
