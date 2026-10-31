// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import type { GitConflictChunk } from '../../preload/types'
import {
  buildMiddle,
  buildProposal,
  diskSegments,
  fromDisk,
  proposedIndexes,
  savedText,
  toDisk
} from './conflictModel.ts'

const clean = (lines: string[]): GitConflictChunk => ({ kind: 'clean', lines, disk: null })
const conflict = (left: string[], base: string[], right: string[]): GitConflictChunk => ({
  kind: 'conflict',
  left,
  base,
  right,
  disk: null
})

// git's merge: a, b, conflict (L | B | R), c, d.
const git = [clean(['a', 'b']), conflict(['L'], ['B'], ['R']), clean(['c', 'd'])]

test('a resolved conflict maps onto the model lines and edits outside it are marked', () => {
  const result = buildProposal(git, 'a2\nb\nX\nY\nc\nd\n', null)
  assert.ok(result)
  assert.equal(result.finalNewline, true)
  assert.deepEqual(result.chunks, [
    { kind: 'clean', lines: ['a2', 'b'], disk: { start: 1, end: 2 } },
    {
      kind: 'conflict',
      left: ['L'],
      base: ['B'],
      right: ['R'],
      disk: {
        start: 3,
        end: 4,
        left: { start: 3, end: 4 },
        base: null,
        right: { start: 3, end: 4 }
      }
    },
    { kind: 'clean', lines: ['c', 'd'], disk: { start: 5, end: 6 } }
  ])
  assert.deepEqual(result.proposal, {
    lines: { 1: ['X', 'Y'] },
    edits: [{ index: 0, start: 0, end: 0 }]
  })

  const middle = buildMiddle(result.chunks, result.proposal)
  assert.equal(middle.text, 'a2\nb\nX\nY\nc\nd\n')
  assert.deepEqual(middle.edits, [{ start: 1, end: 1 }])
  assert.equal(middle.chunks.length, 1)
  assert.equal(middle.chunks[0].proposed, true)
  assert.equal(middle.chunks[0].resolved, false)
  assert.deepEqual(proposedIndexes(result.proposal), new Set([1]))
})

test('the middle maps a proposed conflict to disk line for line, so a note finds its line', () => {
  const result = buildProposal(git, 'a\nb\nX\nY\nc\nd\n', null)
  assert.ok(result)
  const ranges = [
    { start: 1, end: 2 },
    { start: 3, end: 4 },
    { start: 5, end: 6 }
  ]
  const segments = diskSegments(result.chunks, ranges, 'middle', proposedIndexes(result.proposal))
  assert.deepEqual(fromDisk(segments, 4, 4), { fromLine: 4, toLine: 4 })
  assert.deepEqual(toDisk(segments, 3, 3), { startLine: 3, endLine: 3 })
  // Without the proposal set the block maps as a whole, as a marker block does.
  assert.deepEqual(fromDisk(diskSegments(result.chunks, ranges, 'middle'), 4, 4), {
    fromLine: 3,
    toLine: 4
  })
})

test('a conflict the model deleted becomes an empty proposal', () => {
  const result = buildProposal(git, 'a\nb\nc\nd', null)
  assert.ok(result)
  assert.equal(result.finalNewline, false)
  assert.deepEqual(result.chunks[1], {
    kind: 'conflict',
    left: ['L'],
    base: ['B'],
    right: ['R'],
    disk: { start: 3, end: 2, left: { start: 3, end: 2 }, base: null, right: { start: 3, end: 2 } }
  })
  assert.deepEqual(result.proposal, { lines: { 1: [] }, edits: [] })
  assert.equal(buildMiddle(result.chunks, result.proposal).text, 'a\nb\nc\nd\n')
})

test('a marker block the model left stays an open conflict and keeps its git conflict out', () => {
  const twoConflicts = [
    clean(['a']),
    conflict(['L1'], ['B1'], ['R1']),
    clean(['m']),
    conflict(['L2'], ['B2'], ['R2']),
    clean(['z'])
  ]
  const working = 'a\nX1\nm\n<<<<<<< ours\nL2\n=======\nR2\n>>>>>>> theirs\nz\n'
  const block: GitConflictChunk = {
    kind: 'conflict',
    left: ['L2'],
    base: [],
    right: ['R2'],
    disk: { start: 4, end: 8, left: { start: 5, end: 5 }, base: null, right: { start: 7, end: 7 } }
  }
  const workingChunks: GitConflictChunk[] = [
    { kind: 'clean', lines: ['a', 'X1', 'm'], disk: { start: 1, end: 3 } },
    block,
    { kind: 'clean', lines: ['z'], disk: { start: 9, end: 9 } }
  ]
  const result = buildProposal(twoConflicts, working, workingChunks)
  assert.ok(result)
  assert.deepEqual(
    result.chunks.map((chunk) => [chunk.kind, chunk.disk?.start, chunk.disk?.end]),
    [
      ['clean', 1, 1],
      ['conflict', 2, 2],
      ['clean', 3, 3],
      ['conflict', 4, 8],
      ['clean', 9, 9]
    ]
  )
  assert.equal(result.chunks[3], block)
  assert.deepEqual(result.proposal, { lines: { 1: ['X1'] }, edits: [] })
  const middle = buildMiddle(result.chunks, result.proposal)
  assert.equal(middle.text, 'a\nX1\nm\nz\n')
  assert.deepEqual(
    middle.chunks.map((chunk) => [chunk.index, !!chunk.proposed]),
    [
      [1, true],
      [3, false]
    ]
  )
})

test('conflicts the model rewrote together become one proposal holding both', () => {
  const twoConflicts = [
    clean(['a']),
    conflict(['L1'], ['B1'], ['R1']),
    clean(['m']),
    conflict(['L2'], ['B2'], ['R2']),
    clean(['z'])
  ]
  const result = buildProposal(twoConflicts, 'a\nQ\nz\n', null)
  assert.ok(result)
  assert.deepEqual(result.chunks[1], {
    kind: 'conflict',
    left: ['L1', 'm', 'L2'],
    base: ['B1', 'm', 'B2'],
    right: ['R1', 'm', 'R2'],
    disk: { start: 2, end: 2, left: { start: 2, end: 2 }, base: null, right: { start: 2, end: 2 } }
  })
  assert.equal(result.chunks.length, 3)
})

test('saving keeps a waiting proposal as the model wrote it', () => {
  const result = buildProposal(git, 'a\nb\nX\nc\nd\n', null)
  assert.ok(result)
  const middle = buildMiddle(result.chunks, result.proposal)
  const saved = savedText(
    middle.text,
    middle.chunks,
    result.chunks,
    { operation: 'merge', leftRef: 'main', rightRef: 'topic' },
    true,
    false
  )
  assert.deepEqual(saved, { text: 'a\nb\nX\nc\nd\n', resolved: [] })
})

test('CRLF files compare without their carriage returns', () => {
  const result = buildProposal(
    [clean(['a\r']), conflict(['L\r'], ['B\r'], ['R\r']), clean(['c\r'])],
    'a\r\nX\r\nc\r\n',
    null
  )
  assert.ok(result)
  assert.deepEqual(result.proposal, { lines: { 1: ['X'] }, edits: [] })
})
