// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { isTabKind } from './tabKindNames.ts'

test('isTabKind knows current kinds and refuses removed or inherited names', () => {
  assert.equal(isTabKind('sshServer'), true)
  assert.equal(isTabKind('shell'), true)
  assert.equal(isTabKind('remoteFile'), false)
  assert.equal(isTabKind('toString'), false)
})
