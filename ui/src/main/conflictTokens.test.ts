// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  CONFLICT_TOKEN_TTL_MS,
  claimConflictToken,
  claimReply,
  mintConflictToken,
  type ConflictGrant
} from './conflictTokens.ts'

const SCOPE = 'conflict:merge:main:feature'
const TAB = '0b0e6f4c-2a8d-4d7e-9a51-3c1f6f0a9b21'

const START: ConflictGrant = { action: 'start', projectPath: '/p', scope: SCOPE, tabId: TAB }

test('a token claims its grant once and is spent after', () => {
  const token = mintConflictToken(START, 1_000)
  assert.deepEqual(claimConflictToken(token, 1_001), START)
  assert.equal(claimConflictToken(token, 1_002), null)
})

test('a token is good for five minutes and no longer', () => {
  const fresh = mintConflictToken(START, 0)
  assert.deepEqual(claimConflictToken(fresh, CONFLICT_TOKEN_TTL_MS - 1), START)
  const stale = mintConflictToken(START, 0)
  assert.equal(claimConflictToken(stale, CONFLICT_TOKEN_TTL_MS), null)
})

test('an expired claim still spends the token', () => {
  const token = mintConflictToken(START, 0)
  assert.equal(claimConflictToken(token, CONFLICT_TOKEN_TTL_MS + 1), null)
  assert.equal(claimConflictToken(token, 1), null)
})

test('an unknown token claims nothing', () => {
  assert.equal(claimConflictToken('not-a-token'), null)
  assert.equal(claimConflictToken(''), null)
})

test('each token holds only its own grant', () => {
  const close: ConflictGrant = {
    action: 'close',
    projectPath: '/p',
    scope: SCOPE,
    threadId: 'abc123'
  }
  const a = mintConflictToken(START, 0)
  const b = mintConflictToken(close, 0)
  assert.notEqual(a, b)
  assert.deepEqual(claimConflictToken(b, 1), close)
  assert.deepEqual(claimConflictToken(a, 1), START)
})

test('the claim reply carries each action in the shape lgrass reads', () => {
  assert.deepEqual(claimReply(START), {
    action: 'start',
    project_path: '/p',
    scope: SCOPE,
    tab_id: TAB
  })
  const anchor = { path: 'a.txt', startLine: 3, endLine: 4, quote: 'x\ny' }
  assert.deepEqual(
    claimReply({
      action: 'post',
      projectPath: '/p',
      scope: SCOPE,
      messages: [
        { body: 'Keep both.', anchor },
        { body: 'Thanks.', anchor: null }
      ]
    }),
    {
      action: 'post',
      project_path: '/p',
      scope: SCOPE,
      messages: [
        { body: 'Keep both.', anchor },
        { body: 'Thanks.', anchor: null }
      ]
    }
  )
  assert.deepEqual(
    claimReply({ action: 'close', projectPath: '/p', scope: SCOPE, threadId: 'abc123' }),
    { action: 'close', project_path: '/p', scope: SCOPE, thread_id: 'abc123' }
  )
})
