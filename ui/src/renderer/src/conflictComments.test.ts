// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import type { WorkgroupThreadMessage } from '../../preload/types'
import { blockAtLine, sentComments } from './conflictComments.ts'

let nextId = 1
function message(fields: Partial<WorkgroupThreadMessage>): WorkgroupThreadMessage {
  return {
    id: nextId++,
    tabId: '',
    label: 'You',
    body: '',
    createdAt: '',
    author: 'human',
    kind: 'message',
    anchor: null,
    files: [],
    replyTo: null,
    data: null,
    ...fields
  }
}

const anchor = {
  path: 'a.go',
  startLine: 3,
  endLine: 4,
  quote: 'x',
  pane: 'right' as const,
  block: 2
}

test('only the human line comments that name a pane are cards, and replies gather under them', () => {
  const comment = message({ body: 'why?', anchor })
  const chat = message({ body: 'about the block', anchor: { ...anchor, startLine: 0, endLine: 0 } })
  const plain = message({ body: 'hello' })
  const reply = message({
    author: 'tab',
    tabId: 't1',
    label: 'Claude',
    body: 'renamed',
    replyTo: comment.id
  })
  const followUp = message({ body: 'thanks', replyTo: comment.id })
  const noPane = message({ body: 'old note', anchor: { ...anchor, pane: '' } })
  const cards = sentComments([comment, chat, plain, reply, followUp, noPane])
  assert.equal(cards.length, 1)
  assert.equal(cards[0].id, comment.id)
  assert.equal(cards[0].anchor.block, 2)
  assert.deepEqual(
    cards[0].replies.map((r) => [r.author, r.body]),
    [
      ['tab', 'renamed'],
      ['human', 'thanks']
    ]
  )
})

test('a line belongs to the block holding it, else the nearest, else none', () => {
  const blocks = [
    { block: 1, start: 5, end: 8 },
    { block: 2, start: 20, end: 22 }
  ]
  assert.equal(blockAtLine(blocks, 6), 1)
  assert.equal(blockAtLine(blocks, 9), 1)
  assert.equal(blockAtLine(blocks, 18), 2)
  assert.equal(blockAtLine(blocks, 1), 1)
  assert.equal(blockAtLine([], 4), 0)
})
