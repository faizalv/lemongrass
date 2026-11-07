// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { winUnsupportedReason } from './updaterWinScript.ts'

test('winUnsupportedReason needs x64 and an installed copy', () => {
  assert.equal(winUnsupportedReason('x64', true), null)
  assert.match(winUnsupportedReason('arm64', true) ?? '', /x64 only/)
  assert.match(winUnsupportedReason('x64', false) ?? '', /not installed from the setup/)
})
