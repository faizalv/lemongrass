// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import type { GitConflictChunk, GitConflictFile, WorkgroupThreadMessage } from '../../preload/types'
import { buildProposal } from './conflictModel.ts'
import {
  blockDigest,
  fileDecisions,
  fileSettled,
  fileStatusText,
  generalLine,
  matchSides,
  roundDone,
  verdictBatch,
  type BlockCard,
  type Draft
} from './conflictDecisions.ts'

type OkFile = Extract<GitConflictFile, { status: 'ok' }>
type Conflict = Extract<GitConflictChunk, { kind: 'conflict' }>

const conflict = (left: string[], base: string[], right: string[]): Conflict => ({
  kind: 'conflict',
  left,
  base,
  right,
  disk: null
})

// git's merge: a, block 1 (L1 | B1 | R1), b, block 2 (L2 | B2 | R2), c.
const gitChunks: GitConflictChunk[] = [
  { kind: 'clean', lines: ['a'], disk: null },
  conflict(['L1'], ['B1'], ['R1']),
  { kind: 'clean', lines: ['b'], disk: null },
  conflict(['L2'], ['B2'], ['R2']),
  { kind: 'clean', lines: ['c'], disk: null }
]

// The model took R1 for block 1 and left block 2 with its markers.
const working = 'a\nR1\nb\n<<<<<<< ours\nL2\n=======\nR2\n>>>>>>> theirs\nc\n'
const workingChunks: GitConflictChunk[] = [
  { kind: 'clean', lines: ['a', 'R1', 'b'], disk: { start: 1, end: 3 } },
  {
    kind: 'conflict',
    left: ['L2'],
    base: [],
    right: ['R2'],
    disk: { start: 4, end: 8, left: { start: 5, end: 5 }, base: null, right: { start: 7, end: 7 } }
  },
  { kind: 'clean', lines: ['c'], disk: { start: 9, end: 9 } }
]

const file: OkFile = {
  status: 'ok',
  path: 'src/a.js',
  operation: 'merge',
  leftRef: 'main',
  rightRef: 'feature',
  base: { state: 'text', text: '' },
  left: { state: 'text', text: '' },
  right: { state: 'text', text: '' },
  chunks: gitChunks,
  finalNewline: true,
  working,
  workingChunks
}

let nextId = 1
function message(fields: Partial<WorkgroupThreadMessage>): WorkgroupThreadMessage {
  return {
    id: nextId++,
    tabId: 'tab',
    label: 'model',
    body: '',
    createdAt: '',
    author: 'tab',
    kind: 'message',
    anchor: null,
    files: [],
    replyTo: null,
    data: null,
    ...fields
  }
}

function cardsOf(messages: WorkgroupThreadMessage[]): BlockCard[] {
  const mapped = buildProposal(file.chunks!, file.working!, file.workingChunks)
  const decisions = fileDecisions(file, mapped, messages)
  assert.ok(decisions.cards)
  return decisions.cards
}

const decision = message({
  kind: 'decision',
  body: 'feature renamed it',
  data: { path: 'src/a.js', block: 1 }
})
const question = message({
  kind: 'question',
  body: 'which one?',
  data: { path: 'src/a.js', block: 2, options: ['L2', 'R2', 7] }
})

test('a written text equal to a side, or both in either order, reads as taking them', () => {
  const block = conflict(['L1'], [], ['R1', 'R2'])
  assert.deepEqual(matchSides(block, ['L1']), ['left'])
  assert.deepEqual(matchSides(block, ['R1', 'R2']), ['right'])
  assert.deepEqual(matchSides(block, ['L1', 'R1', 'R2']), ['left', 'right'])
  assert.deepEqual(matchSides(block, ['R1', 'R2', 'L1']), ['right', 'left'])
  assert.deepEqual(matchSides(block, ['R1\r', 'R2\r']), ['right'])
  assert.equal(matchSides(block, ['L1', 'R2']), null)
  assert.equal(matchSides(block, []), null)
})

test('a digest tells a block apart from its next version and ignores carriage returns', () => {
  assert.equal(blockDigest(['a', 'b']), blockDigest(['a\r', 'b\r']))
  assert.notEqual(blockDigest(['a', 'b']), blockDigest(['a', 'c']))
  assert.notEqual(blockDigest(['a b']), blockDigest(['a', 'b']))
})

test('each block reads its state from the model report and what the file holds', () => {
  const [first, second] = cardsOf([decision, question])
  assert.deepEqual(first.blocks, [1])
  assert.equal(first.state, 'take')
  assert.deepEqual(first.hold, { kind: 'written', lines: ['R1'], sides: ['right'] })
  assert.equal(first.report?.text, 'feature renamed it')
  assert.deepEqual(second.blocks, [2])
  assert.equal(second.state, 'question')
  assert.deepEqual(second.report?.options, ['L2', 'R2'])
})

test('a report the file contradicts, or a block written with no report, is a mismatch', () => {
  const [first, second] = cardsOf([
    message({ kind: 'decision', data: { path: 'src/a.js', block: 2 } })
  ])
  assert.equal(first.state, 'mismatch')
  assert.equal(first.mismatch, 'unreported')
  assert.equal(second.state, 'mismatch')
  assert.equal(second.mismatch, 'markersLeft')
})

test('an untouched block is open, and data from another file or a malformed one is ignored', () => {
  const cards = cardsOf([
    message({ kind: 'question', data: { path: 'src/b.js', block: 2 } }),
    message({ kind: 'question', data: { path: 'src/a.js', block: '2' } }),
    message({ kind: 'decision', author: 'human', data: { path: 'src/a.js', block: 2 } })
  ])
  assert.equal(cards[1].state, 'open')
})

test('a verdict after the report settles a block, until its text changes', () => {
  const accept = (digest: string): WorkgroupThreadMessage =>
    message({
      kind: 'verdicts',
      author: 'human',
      data: { path: 'src/a.js', verdicts: [{ block: 1, verdict: 'accepted', digest }] }
    })
  assert.equal(cardsOf([decision, accept(blockDigest(['R1']))])[0].state, 'accepted')
  assert.equal(cardsOf([decision, accept(blockDigest(['L1']))])[0].state, 'changed')
  const rejected = message({
    kind: 'verdicts',
    author: 'human',
    data: { path: 'src/a.js', verdicts: [{ block: 1, verdict: 'rejected', text: 'keep main' }] }
  })
  assert.equal(cardsOf([decision, rejected])[0].state, 'waiting')
  // A new decision after the rejection is a take to review again.
  const again = message({ kind: 'decision', data: { path: 'src/a.js', block: 1 } })
  assert.equal(cardsOf([decision, rejected, again])[0].state, 'take')
})

test('drafts become one verdict per block, and only drafts that still fit their card', () => {
  const [first, second] = cardsOf([decision, question])
  const drafts: Record<string, Draft> = {
    [first.key]: { kind: 'accept', digest: first.digest },
    [second.key]: { kind: 'answer', text: ' R2 ', question: second.report!.id }
  }
  assert.deepEqual(verdictBatch([first, second], drafts, undefined), [
    { block: 1, verdict: 'accepted', digest: first.digest },
    { block: 2, verdict: 'answered', text: 'R2', question: second.report!.id }
  ])
  assert.equal(fileSettled([first, second], drafts, undefined), false)

  const stale = { [first.key]: { kind: 'accept' as const, digest: 'old' } }
  assert.deepEqual(verdictBatch([first], stale, undefined), [])
  const wrongQuestion = { [second.key]: { kind: 'answer' as const, text: 'x', question: 999 } }
  assert.deepEqual(verdictBatch([second], wrongQuestion, undefined), [])

  const manual = new Map([[2, 'd2']])
  assert.deepEqual(verdictBatch([second], {}, manual), [
    { block: 2, verdict: 'resolved', digest: 'd2' }
  ])
  assert.equal(fileSettled([first, second], drafts, manual), true)
})

test('the general line, the end of a round and the file status read the thread', () => {
  const human = message({ author: 'human', body: 'go on' })
  const line = message({ body: 'I also fixed an import.' })
  const submit = message({ kind: 'submit' })
  assert.equal(generalLine([line, human, submit])?.body, 'I also fixed an import.')
  assert.equal(roundDone([line, human, submit]), true)
  assert.equal(roundDone([submit, message({ author: 'human', body: 'not yet' })]), false)

  assert.equal(fileStatusText(cardsOf([decision, question])), '1 question')
  assert.equal(fileStatusText(cardsOf([decision])), '1 to review')
  assert.equal(fileStatusText(null), null)
})
