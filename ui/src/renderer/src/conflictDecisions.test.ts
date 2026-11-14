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
  contextAround,
  countStates,
  generalLine,
  handOverAsked,
  isQueued,
  listLabel,
  listStateOf,
  matchSides,
  nextStep,
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

const verdictsMessage = (
  path: string,
  verdicts: { block: number; verdict: string; text?: string; digest?: string }[]
): WorkgroupThreadMessage =>
  message({ author: 'human', kind: 'verdicts', data: { path, verdicts } })

test('a hand-over that asked about blocks is read from the opening verdict batches only', () => {
  const resolved = verdictsMessage('src/a.js', [{ block: 1, verdict: 'resolved', digest: 'd' }])
  const asked = verdictsMessage('src/a.js', [{ block: 2, verdict: 'asked', text: 'look' }])
  const opening = handOverAsked([resolved, asked])
  assert.deepEqual([...(opening?.get('src/a.js') ?? [])], [2])
  assert.equal(handOverAsked([resolved]), null)
  assert.equal(handOverAsked([]), null)
  // The model reported first, so a later ask says nothing about how the conflict was handed over.
  assert.equal(handOverAsked([decision, asked]), null)
})

test('a block no one was asked about, or the model never reported, is untouched', () => {
  const [first, second] = cardsOf([decision])
  assert.equal(first.state, 'take')
  assert.equal(second.state, 'open')
  const asked = new Map([['src/a.js', new Set([1])]])
  assert.equal(listStateOf(second, false, null, false), 'model')
  assert.equal(listStateOf(second, false, asked, false), 'untouched')
  assert.equal(listStateOf(second, false, new Map([['src/a.js', new Set([2])]]), false), 'model')
  assert.equal(listStateOf(second, false, null, true), 'untouched')
  assert.equal(listLabel('untouched', second), 'Untouched')
})

test('a decided card is queued, a take to review needs the human, a settled one is done', () => {
  const [take, ask] = cardsOf([decision, question])
  assert.equal(listStateOf(take, false, null, false), 'needs')
  assert.equal(listLabel('needs', take), 'Review')
  assert.equal(listLabel('needs', ask), 'Question')
  const accept: Draft = { kind: 'accept', digest: take.digest }
  assert.equal(isQueued(take, accept, undefined), true)
  assert.equal(isQueued(take, { kind: 'accept', digest: 'old' }, undefined), false)
  assert.equal(isQueued(take, { kind: 'manual' }, undefined), false)
  assert.equal(isQueued(ask, undefined, new Map([[2, 'd2']])), true)
  assert.equal(listStateOf(take, true, null, false), 'queued')
  const accepted = cardsOf([
    decision,
    verdictsMessage('src/a.js', [{ block: 1, verdict: 'accepted', digest: take.digest }])
  ])[0]
  assert.equal(listStateOf(accepted, false, null, false), 'done')
})

test('the next step says whose move it is from the counts', () => {
  const counts = countStates
  assert.equal(
    nextStep(counts(['needs', 'untouched', 'untouched'])),
    '1 block needs you. 2 untouched.'
  )
  assert.equal(nextStep(counts(['needs', 'needs'])), '2 blocks need you.')
  assert.equal(nextStep(counts(['queued', 'done'])), '1 decided and waiting for Send.')
  assert.equal(
    nextStep(counts(['untouched', 'done'])),
    'Nothing needs you. 1 untouched: ask an agent or resolve them yourself.'
  )
  assert.equal(nextStep(counts(['model'])), 'Nothing needs you while the agent works.')
  assert.equal(nextStep(counts(['done', 'done'])), 'Everything is settled.')
})

test('a card carries the clean lines around its block, for each side and for the result', () => {
  const [first, second] = cardsOf([])
  assert.deepEqual(first.leftContext, { before: ['a'], after: ['b'], start: null })
  assert.deepEqual(second.rightContext, { before: ['b'], after: ['c'], start: null })
  assert.deepEqual(second.resultContext.after, ['c'])
  assert.ok(first.resultContext.before.length > 0)
})

test('with the stage files located, each side numbers its lines and stops at a neighbour conflict', () => {
  const stage = (text: string): OkFile['left'] => ({ state: 'text', text })
  const withStages: OkFile = {
    ...file,
    left: stage('a\nL1\nb\nL2\nc\n'),
    right: stage('a\nR1\nb\nR2\nc\n')
  }
  const range = (start: number, end = start): { start: number; end: number } => ({ start, end })
  const located = {
    left: [range(1), range(2), range(3), range(4), range(5)],
    right: [range(1), range(2), range(3), range(4), range(5)]
  }
  const mapped = buildProposal(file.chunks!, file.working!, file.workingChunks)
  const cards = fileDecisions(withStages, mapped, [], located).cards!
  assert.deepEqual(cards[0].leftContext, { before: ['a'], after: ['b'], start: 1 })
  // Block 2 sits after clean b, so its context is b alone and numbering starts there.
  assert.deepEqual(cards[1].leftContext, { before: ['b'], after: ['c'], start: 3 })
  assert.equal(cards[1].rightContext.start, 3)
})

test('context is cut to the nearest lines and is empty beside another conflict', () => {
  const lines = Array.from({ length: 12 }, (_, i) => `l${i + 1}`)
  const chunks: GitConflictChunk[] = [
    { kind: 'clean', lines, disk: null },
    conflict(['L'], ['B'], ['R']),
    { kind: 'clean', lines, disk: null },
    conflict(['L'], ['B'], ['R'])
  ]
  assert.deepEqual(contextAround(chunks, 1, 3), {
    before: ['l10', 'l11', 'l12'],
    after: ['l1', 'l2', 'l3'],
    start: null
  })
  assert.equal(contextAround(chunks, 1, 10).before.length, 10)
  assert.deepEqual(contextAround(chunks, 3, 3).after, [])
  assert.deepEqual(contextAround([conflict([], [], []), chunks[1]], 1, 3).before, [])
})
