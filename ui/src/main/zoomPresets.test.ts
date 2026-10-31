// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { zoomPresets } from './zoomPresets.ts'

test('a wider screen offers larger zooms', () => {
  assert.deepEqual(zoomPresets(1920), [0.75, 1, 1.25, 1.5])
  assert.deepEqual(zoomPresets(2560), [0.75, 1, 1.25, 1.5, 1.75, 2])
  assert.equal(zoomPresets(3840).at(-1), 3)
})

test('a narrow screen still offers 100%', () => {
  assert.deepEqual(zoomPresets(1366), [0.75, 1])
  assert.deepEqual(zoomPresets(1024), [0.75, 1])
})
