// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import type { WorkgroupThreadMessage } from '../../preload/types'
import { fileStatus, latestSubmit, mergeMessages, notesFor } from './conflictThreadModel.ts'

function message(
  id: number,
  kind: WorkgroupThreadMessage['kind'],
  extra: Partial<WorkgroupThreadMessage> = {}
): WorkgroupThreadMessage {
  return {
    id,
    tabId: 't1',
    label: 'Claude Code 1',
    body: `m${id}`,
    createdAt: '',
    author: 'tab',
    kind,
    anchor: null,
    files: [],
    ...extra
  }
}

const anchor = (path: string, line: number): WorkgroupThreadMessage['anchor'] => ({
  path,
  startLine: line,
  endLine: line,
  quote: 'x'
})

test('mergeMessages keeps paged messages, replaces by id and sorts oldest first', () => {
  const held = [message(2, 'message'), message(1, 'message')]
  const merged = mergeMessages(held, [message(3, 'note'), message(2, 'message', { body: 'new' })])
  assert.deepEqual(
    merged.map((m) => [m.id, m.body]),
    [
      [1, 'm1'],
      [2, 'new'],
      [3, 'm3']
    ]
  )
})

test('the latest submit wins and decides which files are proposed', () => {
  const messages = [
    message(1, 'note', { anchor: anchor('a.ts', 3) }),
    message(2, 'submit', { files: ['a.ts', 'b.ts'] }),
    message(3, 'note', { anchor: anchor('a.ts', 9) }),
    message(4, 'note', { anchor: anchor('b.ts', 1) }),
    message(5, 'submit', { files: ['a.ts'] })
  ]
  assert.equal(latestSubmit(messages)?.id, 5)
  assert.deepEqual(
    notesFor(messages, 'a.ts').map((m) => m.id),
    [1, 3]
  )
  assert.deepEqual(fileStatus(messages, 'a.ts', 0), { kind: 'proposed', notes: 2 })
  assert.equal(fileStatus(messages, 'b.ts', 0), null)
  assert.deepEqual(fileStatus(messages, 'a.ts', 2), { kind: 'markers', blocks: 2 })
})

test('nothing is proposed before the first submit', () => {
  assert.equal(latestSubmit([message(1, 'message')]), null)
  assert.equal(fileStatus([message(1, 'note', { anchor: anchor('a.ts', 1) })], 'a.ts', 0), null)
})
