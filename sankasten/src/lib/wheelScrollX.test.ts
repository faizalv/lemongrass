// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { wheelScrollX } from './wheelScrollX.ts'

test('a vertical wheel scrolls an overflowing strip sideways by its pixels', () => {
  assert.equal(wheelScrollX({ deltaX: 0, deltaY: 120, deltaMode: 0 }, 300, 900), 120)
  assert.equal(wheelScrollX({ deltaX: 0, deltaY: -40, deltaMode: 0 }, 300, 900), -40)
})

test('line and page deltas are turned into pixels', () => {
  assert.equal(wheelScrollX({ deltaX: 0, deltaY: 3, deltaMode: 1 }, 300, 900), 48)
  assert.equal(wheelScrollX({ deltaX: 0, deltaY: 1, deltaMode: 2 }, 300, 900), 300)
})

test('a strip that fits leaves the wheel to the page', () => {
  assert.equal(wheelScrollX({ deltaX: 0, deltaY: 120, deltaMode: 0 }, 300, 300), null)
})

test('a sideways swipe is left to the browser', () => {
  assert.equal(wheelScrollX({ deltaX: 60, deltaY: 10, deltaMode: 0 }, 300, 900), null)
  assert.equal(wheelScrollX({ deltaX: 30, deltaY: 30, deltaMode: 0 }, 300, 900), null)
})
