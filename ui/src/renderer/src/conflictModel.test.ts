// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import type { GitConflictChunk } from '../../preload/types'
import {
  applyText,
  buildMiddle,
  collapseSpans,
  diskSegments,
  fromDisk,
  isCrlf,
  locateChunks,
  mapScroll,
  middleRanges,
  takeText,
  toDisk
} from './conflictModel.ts'

// Working file on disk:
//  1 a
//  2 <<<<<<< ours
//  3 L1
//  4 =======
//  5 R1
//  6 R2
//  7 >>>>>>> theirs
//  8 b
//  9 c
const chunks: GitConflictChunk[] = [
  { kind: 'clean', lines: ['a'], disk: { start: 1, end: 1 } },
  {
    kind: 'conflict',
    left: ['L1'],
    base: ['B1'],
    right: ['R1', 'R2'],
    disk: {
      start: 2,
      end: 7,
      left: { start: 3, end: 3 },
      base: null,
      right: { start: 5, end: 6 }
    }
  },
  { kind: 'clean', lines: ['b', 'c'], disk: { start: 8, end: 9 } }
]

test('the middle holds the base of each conflict, unresolved', () => {
  const middle = buildMiddle(chunks)
  assert.equal(middle.text, 'a\nB1\nb\nc\n')
  assert.deepEqual(middle.chunks, [{ index: 1, from: 2, to: 5, resolved: false }])
})

test('take both puts left then right', () => {
  assert.equal(takeText(chunks[1], 'both'), 'L1\nR1\nR2\n')
  assert.equal(takeText(chunks[1], 'right'), 'R1\nR2\n')
})

test('a conflict with no base starts empty in the middle', () => {
  const middle = buildMiddle([
    { kind: 'conflict', left: ['x'], base: [], right: ['y'], disk: null }
  ])
  assert.deepEqual(middle, { text: '', chunks: [{ index: 0, from: 0, to: 0, resolved: false }] })
})

test('apply drops the newline the file did not have and restores CRLF', () => {
  assert.equal(applyText('a\nb\n', false, false), 'a\nb')
  assert.equal(applyText('a\nb\n', true, true), 'a\r\nb\r\n')
  assert.equal(isCrlf([{ kind: 'clean', lines: ['a\r'], disk: null }]), true)
})

test('chunks are found in a side file whose clean lines the other side changed', () => {
  // The right side changed the clean "b" to "b2" in the merged result, so the left file still has b.
  const merged: GitConflictChunk[] = [
    chunks[0],
    chunks[1],
    { kind: 'clean', lines: ['b2', 'c'], disk: { start: 8, end: 9 } }
  ]
  assert.deepEqual(locateChunks(merged, 'left', 'a\nL1\nb\nc\n'), [
    { start: 1, end: 1 },
    { start: 2, end: 2 },
    { start: 3, end: 4 }
  ])
  assert.deepEqual(locateChunks(merged, 'right', 'a\nR1\nR2\nb2\nc\n'), [
    { start: 1, end: 1 },
    { start: 2, end: 3 },
    { start: 4, end: 5 }
  ])
})

test('middle ranges fill the gaps around the conflicts', () => {
  assert.deepEqual(middleRanges(chunks, [{ start: 2, end: 3 }], 5), [
    { start: 1, end: 1 },
    { start: 2, end: 3 },
    { start: 4, end: 5 }
  ])
})

test('pane lines map to disk lines and back', () => {
  const right = diskSegments(
    chunks,
    [
      { start: 1, end: 1 },
      { start: 2, end: 3 },
      { start: 4, end: 5 }
    ],
    'right'
  )
  assert.deepEqual(toDisk(right, 3, 4), { startLine: 6, endLine: 8 })
  assert.deepEqual(fromDisk(right, 5, 5), { fromLine: 2, toLine: 2 })

  const middle = diskSegments(
    chunks,
    [
      { start: 1, end: 1 },
      { start: 2, end: 2 },
      { start: 3, end: 4 }
    ],
    'middle'
  )
  // A line in a conflict maps to its whole marker block.
  assert.deepEqual(toDisk(middle, 2, 2), { startLine: 2, endLine: 7 })
  assert.deepEqual(toDisk(middle, 4, 4), { startLine: 9, endLine: 9 })
  assert.equal(
    toDisk(diskSegments([{ ...chunks[0], disk: null }], [{ start: 1, end: 1 }], 'left'), 1, 1),
    null
  )
})

test('scroll interpolates between chunk boundaries', () => {
  const a = [
    { start: 1, end: 1 },
    { start: 2, end: 2 },
    { start: 3, end: 4 }
  ]
  const b = [
    { start: 1, end: 1 },
    { start: 2, end: 5 },
    { start: 6, end: 7 }
  ]
  assert.equal(mapScroll(a, b, 4, 7, 1), 1)
  assert.equal(mapScroll(a, b, 4, 7, 1.5), 3)
  assert.equal(mapScroll(a, b, 4, 7, 3), 6)
})

test('focus hides clean lines beyond the context around conflicts', () => {
  // Clean 1 to 10, a conflict at 11 to 12, clean 13 to 20, a conflict at 21, clean 22 to 23.
  const clean = [
    { start: 1, end: 10 },
    { start: 13, end: 20 },
    { start: 22, end: 23 }
  ]
  assert.deepEqual(collapseSpans(clean, [], 23), [
    { start: 1, end: 7 },
    { start: 16, end: 17 }
  ])
  // A commented line stays visible and splits the span around it.
  assert.deepEqual(collapseSpans(clean, [{ start: 4, end: 4 }], 23), [
    { start: 1, end: 3 },
    { start: 5, end: 7 },
    { start: 16, end: 17 }
  ])
})
