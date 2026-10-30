// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { EditorState } from '@codemirror/state'
import { undo } from '@codemirror/commands'
import { chunkAtCursor, chunkReplacements, middleChunks, middleState } from './conflictEditor.ts'
import type { MiddleChunk } from './conflictModel.ts'

const open = { resolved: false, left: 'open', right: 'open', taken: [] } as const

// a, then conflict 1 holding its base B1, then b.
function state(conflict = 'B1\n'): EditorState {
  const to = 2 + conflict.length
  return middleState(
    `a\n${conflict}b\n`,
    [{ index: 1, from: 2, to, ...open, taken: [] }],
    () => {},
    []
  )
}

function chunk(state: EditorState): MiddleChunk {
  return state.field(middleChunks)[0]
}

test('a decision replaces the conflict and records it', () => {
  const start = state()
  const decided: MiddleChunk = { ...chunk(start), right: 'taken', taken: ['right'] }
  const next = start.update(chunkReplacements(start, [{ chunk: decided, text: 'R1\nR2\n' }])!).state
  assert.equal(next.doc.toString(), 'a\nR1\nR2\nb\n')
  assert.deepEqual(chunk(next), { ...decided, from: 2, to: 8 })
})

test('several conflicts are rewritten in one step', () => {
  const start = middleState(
    'B1\nc\nB2\n',
    [
      { index: 0, from: 0, to: 3, ...open, taken: [] },
      { index: 2, from: 5, to: 8, ...open, taken: [] }
    ],
    () => {},
    []
  )
  const [first, second] = start.field(middleChunks)
  const next = start.update(
    chunkReplacements(start, [
      { chunk: { ...second, resolved: true }, text: 'X\nY\n' },
      { chunk: { ...first, resolved: true }, text: '' }
    ])!
  ).state
  assert.equal(next.doc.toString(), 'c\nX\nY\n')
  assert.deepEqual(
    next.field(middleChunks).map(({ from, to }) => [from, to]),
    [
      [0, 0],
      [2, 6]
    ]
  )
})

test('an edit inside a conflict resolves it, an edit beside it does not', () => {
  const start = state()
  const beside = start.update({ changes: { from: 5, insert: 'x' } }).state
  assert.equal(chunk(beside).resolved, false)
  assert.equal(chunk(beside).to, 5)
  const inside = start.update({ changes: { from: 3, insert: 'x' } }).state
  assert.deepEqual(chunk(inside), { index: 1, from: 2, to: 6, ...open, taken: [], resolved: true })
})

test('text typed into an empty conflict lands inside it', () => {
  const start = state('')
  assert.equal(chunkAtCursor(start.update({ selection: { anchor: 2 } }).state), 1)
  const typed = start.update({ changes: { from: 2, insert: 'n\n' } }).state
  assert.deepEqual([chunk(typed).from, chunk(typed).to, chunk(typed).resolved], [2, 4, true])
})

test('undoing a decision puts the conflict back as it was', () => {
  let current = state()
  const decided: MiddleChunk = { ...chunk(current), left: 'taken', taken: ['left'] }
  current = current.update(chunkReplacements(current, [{ chunk: decided, text: 'L1\n' }])!).state
  undo({ state: current, dispatch: (tr) => (current = tr.state) })
  assert.equal(current.doc.toString(), 'a\nB1\nb\n')
  assert.deepEqual(chunk(current), { index: 1, from: 2, to: 5, ...open, taken: [] })
})
