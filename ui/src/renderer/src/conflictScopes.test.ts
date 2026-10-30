// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  activeConflictKind,
  conflictScope,
  endedConflictScopes,
  parseConflictScope
} from './conflictScopes.ts'
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

test('scopes end when nothing is stopped or another kind is', () => {
  const merge = conflictScope('merge', 'main', 'feature')
  const stash = conflictScope('stash', 'stashed changes', 'main')
  const scopes = [merge, stash, 'web:page']
  assert.deepEqual(endedConflictScopes(scopes, null), [merge, stash])
  assert.deepEqual(endedConflictScopes(scopes, 'merge'), [stash])
  assert.deepEqual(endedConflictScopes(scopes, 'rebase'), [merge, stash])
})

test('a scope key parses back to its operation and refs', () => {
  assert.deepEqual(parseConflictScope(conflictScope('rebase', 'feature', 'origin/main')), {
    operation: 'rebase',
    leftRef: 'feature',
    rightRef: 'origin/main'
  })
  assert.equal(parseConflictScope('web:page'), null)
  assert.equal(parseConflictScope('conflict:pick:a:b'), null)
})
