// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import type { WorkgroupThreadMessage } from '../../preload/types'
import {
  chatAwaitsAnswer,
  chatMessageFor,
  chatMessages,
  contextLine,
  newestId,
  scopeChip,
  unreadCount
} from './conflictChat.ts'
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

test('the chat waits on the agent after a line of the human, until the agent posts anything', () => {
  const asked = message({ body: 'why?' })
  const verdicts = message({ kind: 'verdicts' })
  const report = message({ author: 'tab', kind: 'decision', tabId: 't1' })
  const answer = message({ author: 'tab', tabId: 't1' })
  assert.equal(chatAwaitsAnswer([]), false)
  assert.equal(chatAwaitsAnswer([verdicts]), false)
  assert.equal(chatAwaitsAnswer([asked]), true)
  assert.equal(chatAwaitsAnswer([asked, verdicts]), true)
  assert.equal(chatAwaitsAnswer([asked, report]), false)
  assert.equal(chatAwaitsAnswer([asked, answer, message({ body: 'and?' })]), true)
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

test('the Thread count is the agent messages in the chat after the newest one seen', () => {
  const asked = message({ id: 10 })
  const answer = message({ id: 11, author: 'tab', tabId: 't1' })
  const report = message({ id: 12, author: 'tab', tabId: 't1', kind: 'decision' })
  const reply = message({ id: 13, author: 'tab', tabId: 't1', replyTo: 10 })
  const all = [asked, answer, report, reply]
  assert.equal(unreadCount(all, 0), 2)
  assert.equal(unreadCount(all, 11), 1)
  assert.equal(unreadCount(all, newestId(all)), 0)
  assert.equal(newestId([]), 0)
})
