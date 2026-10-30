// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { EditorState } from '@codemirror/state'
import { undo } from '@codemirror/commands'
import { chunkAtCursor, chunkReplacement, middleChunks, middleState } from './conflictEditor.ts'

// a, then conflict 1 holding its base B1, then b.
function state(conflict = 'B1\n'): EditorState {
  const to = 2 + conflict.length
  return middleState(
    `a\n${conflict}b\n`,
    [{ index: 1, from: 2, to, resolved: false }],
    () => {},
    () => () => {},
    []
  )
}

test('a take replaces the conflict and resolves it', () => {
  const start = state()
  const next = start.update(chunkReplacement(start, 1, 'L1\nR1\n', true)!).state
  assert.equal(next.doc.toString(), 'a\nL1\nR1\nb\n')
  assert.deepEqual(next.field(middleChunks), [{ index: 1, from: 2, to: 8, resolved: true }])
})

test('an edit inside a conflict resolves it, an edit beside it does not', () => {
  const start = state()
  const beside = start.update({ changes: { from: 5, insert: 'x' } }).state
  assert.equal(beside.field(middleChunks)[0].resolved, false)
  assert.equal(beside.field(middleChunks)[0].to, 5)
  const inside = start.update({ changes: { from: 3, insert: 'x' } }).state
  assert.deepEqual(inside.field(middleChunks)[0], { index: 1, from: 2, to: 6, resolved: true })
})

test('text typed into an empty conflict lands inside it', () => {
  const start = state('')
  assert.equal(chunkAtCursor(start.update({ selection: { anchor: 2 } }).state), 1)
  const typed = start.update({ changes: { from: 2, insert: 'n\n' } }).state
  assert.deepEqual(typed.field(middleChunks)[0], { index: 1, from: 2, to: 4, resolved: true })
})

test('undoing a take makes the conflict unresolved again', () => {
  let current = state()
  current = current.update(chunkReplacement(current, 1, 'L1\n', true)!).state
  undo({ state: current, dispatch: (tr) => (current = tr.state) })
  assert.equal(current.doc.toString(), 'a\nB1\nb\n')
  assert.deepEqual(current.field(middleChunks)[0], { index: 1, from: 2, to: 5, resolved: false })
})
