// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { formatAge } from './relativeAge.ts'

const at = '2026-10-30T08:00:00Z'
const base = new Date(at).getTime()
const minutes = (n: number): number => base + n * 60 * 1000

test('ages read in words from a moment to days', () => {
  assert.equal(formatAge(at, base + 20 * 1000), 'a moment ago')
  assert.equal(formatAge(at, minutes(1)), 'a minute ago')
  assert.equal(formatAge(at, minutes(42)), '42 minutes ago')
  assert.equal(formatAge(at, minutes(60)), 'an hour ago')
  assert.equal(formatAge(at, minutes(5 * 60)), '5 hours ago')
  assert.equal(formatAge(at, minutes(30 * 60)), 'yesterday')
  assert.equal(formatAge(at, minutes(4 * 24 * 60)), '4 days ago')
})

test('a week or more shows the date, and a future time reads as a moment ago', () => {
  assert.equal(formatAge(at, minutes(8 * 24 * 60)), new Date(at).toLocaleDateString())
  assert.equal(formatAge(at, base - 5000), 'a moment ago')
})
