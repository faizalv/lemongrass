// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { CAPSULE_HUES, capsuleHue, leastUsedHue, type CapsuleHue } from './capsuleHue.ts'

test('one content always gets the same hue', () => {
  assert.equal(capsuleHue('staging-web'), capsuleHue('staging-web'))
})

test('every hue comes from the palette', () => {
  for (const name of ['', 'a', 'build-01', 'db.internal', 'ünïcode']) {
    assert.ok(CAPSULE_HUES.includes(capsuleHue(name)))
  }
})

test('different names spread over more than one hue', () => {
  const names = Array.from({ length: 40 }, (_, i) => `server-${i}`)
  assert.ok(new Set(names.map(capsuleHue)).size > 3)
})

test('a kept hue avoids the ones taken until the palette runs out', () => {
  const used: CapsuleHue[] = []
  for (let i = 0; i < CAPSULE_HUES.length; i++) used.push(leastUsedHue(`project-${i}`, used))
  assert.equal(new Set(used).size, CAPSULE_HUES.length)
  const next = leastUsedHue('one-more', used)
  assert.ok(CAPSULE_HUES.includes(next))
})

test('a kept hue is the content hash when that hue is free', () => {
  assert.equal(leastUsedHue('staging-web', []), capsuleHue('staging-web'))
})
