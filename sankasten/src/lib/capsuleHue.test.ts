// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { CAPSULE_HUES, capsuleHue } from './capsuleHue.ts'

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
