// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { activeConflictKind } from './conflictScopes.ts'
import type { GitFile, GitStatus } from '../../preload/types'

function status(operation: GitStatus['operation'], conflicted: boolean): GitStatus {
  const files = conflicted ? [{ path: 'a.txt', group: 'conflicted' } as GitFile] : []
  return { operation, files } as GitStatus
}

test('the active kind follows the stopped operation, else conflicted files mean stash', () => {
  assert.equal(activeConflictKind(null), undefined)
  assert.equal(activeConflictKind(status('merge', true)), 'merge')
  assert.equal(activeConflictKind(status('rebase', false)), 'rebase')
  assert.equal(activeConflictKind(status(null, true)), 'stash')
  assert.equal(activeConflictKind(status(null, false)), null)
})
