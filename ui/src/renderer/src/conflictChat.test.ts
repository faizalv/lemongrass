// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import type { WorkgroupThreadMessage } from '../../preload/types'
import { chatMessageFor, chatMessages, contextLine, inScope, scopeChip } from './conflictChat.ts'
import { commentMessages } from './conflictComments.ts'

let nextId = 1
function message(fields: Partial<WorkgroupThreadMessage>): WorkgroupThreadMessage {
  return {
    id: nextId++,
    tabId: '',
    label: 'You',
    body: 'x',
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

const at = (path: string, block: number): WorkgroupThreadMessage['anchor'] => ({
  path,
  startLine: 0,
  endLine: 0,
  quote: '',
  pane: '',
  block
})

test('the chat lists plain messages only, never reports or verdicts', () => {
  const kinds = ['message', 'decision', 'question', 'verdicts', 'submit', 'note'] as const
  const all = kinds.map((kind) => message({ kind }))
  assert.deepEqual(
    chatMessages(all).map((m) => m.kind),
    ['message']
  )
})

test('a block scope keeps its block, the file as a whole and general messages', () => {
  const general = message({})
  const sameBlock = message({ anchor: at('src/a.js', 2) })
  const otherBlock = message({ anchor: at('src/a.js', 3) })
  const wholeFile = message({ anchor: at('src/a.js', 0) })
  const otherFile = message({ anchor: at('src/b.js', 2) })
  const scope = { path: 'src/a.js', block: 2 }
  assert.deepEqual(
    [general, sameBlock, otherBlock, wholeFile, otherFile].map((m) => inScope(m, scope)),
    [true, true, false, true, false]
  )
  const file = { path: 'src/a.js', block: null }
  assert.deepEqual(
    [general, sameBlock, otherBlock, wholeFile, otherFile].map((m) => inScope(m, file)),
    [true, true, true, true, false]
  )
})

test('the chip and the context line name the file and the block', () => {
  assert.equal(scopeChip(message({ anchor: at('src/a.js', 2) })), 'a.js, block 2')
  assert.equal(scopeChip(message({ anchor: at('src/a.js', 0) })), 'a.js')
  assert.equal(scopeChip(message({})), null)
  assert.equal(contextLine({ path: 'src/a.js', block: 2 }), 'Asking about a.js, block 2')
  assert.equal(contextLine({ path: 'src/a.js', block: null }), 'Asking about a.js')
})

test('a chat line is anchored to its scope with no lines, and a comment keeps its place', () => {
  assert.deepEqual(chatMessageFor({ path: 'src/a.js', block: 2 }, ' why? '), {
    body: 'why?',
    anchor: { path: 'src/a.js', startLine: 0, endLine: 0, quote: '', pane: '', block: 2 }
  })
  assert.equal(chatMessageFor({ path: 'src/a.js', block: null }, 'x').anchor?.block, 0)
  const messages = commentMessages([
    {
      id: 'c1',
      path: 'src/a.js',
      block: 2,
      pane: 'left',
      startLine: 3,
      endLine: 4,
      quote: 'q',
      comment: 'hm'
    },
    {
      id: 'c2',
      path: 'src/a.js',
      block: 2,
      pane: 'left',
      startLine: 3,
      endLine: 4,
      quote: 'q',
      comment: 'ok',
      replyTo: 9
    }
  ])
  assert.equal(messages[0].anchor?.pane, 'left')
  assert.equal(messages[0].replyTo, undefined)
  assert.equal(messages[1].replyTo, 9)
})
