// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { formatElapsed, spanText } from './elapsed.ts'

test('elapsed time reads like a clock', () => {
  assert.equal(formatElapsed(0), '0:00')
  assert.equal(formatElapsed(7_400), '0:07')
  assert.equal(formatElapsed(754_000), '12:34')
  assert.equal(formatElapsed(3_723_000), '1:02:03')
  assert.equal(formatElapsed(-5_000), '0:00')
})

test('spanText reads in whole minutes, never under one', () => {
  assert.equal(spanText(10_000), '1 min')
  assert.equal(spanText(5 * 60_000), '5 min')
  assert.equal(spanText(60 * 60_000), '1 h')
  assert.equal(spanText(72 * 60_000), '1 h 12 min')
})
