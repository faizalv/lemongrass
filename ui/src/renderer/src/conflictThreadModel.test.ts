// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import type { WorkgroupThreadMessage } from '../../preload/types'
import {
  chipText,
  fileStatus,
  fileStatusText,
  firstLine,
  isQuestionNote,
  latestSubmit,
  mergeMessages,
  notesFor,
  startsRun
} from './conflictThreadModel.ts'

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
    replyTo: null,
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
  assert.deepEqual(fileStatus(messages, 'a.ts', 2), { kind: 'markers', blocks: 2, questions: 0 })
})

test('a note quoting a marker block is a question, and the file status counts it', () => {
  const block = {
    path: 'a.ts',
    startLine: 2,
    endLine: 6,
    quote: '<<<<<<< ours\nx\n=======\ny\n>>>>>>> theirs'
  }
  const messages = [
    message(1, 'note', { anchor: block }),
    message(2, 'note', { anchor: anchor('a.ts', 9) }),
    message(3, 'submit', { files: ['a.ts'] })
  ]
  assert.equal(isQuestionNote(messages[0]), true)
  assert.equal(isQuestionNote(messages[1]), false)
  assert.equal(isQuestionNote(message(4, 'message', { anchor: block })), false)
  const status = fileStatus(messages, 'a.ts', 1)
  assert.deepEqual(status, { kind: 'markers', blocks: 1, questions: 1 })
  assert.equal(fileStatusText(status!), '1 needs your call')
  assert.equal(fileStatusText({ kind: 'markers', blocks: 3, questions: 2 }), '2 need your call')
  assert.equal(fileStatusText({ kind: 'markers', blocks: 3, questions: 0 }), 'markers left')
})

test('nothing is proposed before the first submit', () => {
  assert.equal(latestSubmit([message(1, 'message')]), null)
  assert.equal(fileStatus([message(1, 'note', { anchor: anchor('a.ts', 1) })], 'a.ts', 0), null)
})

test('a chip shows the file and the first code line, skipping marker lines', () => {
  assert.equal(
    chipText({ path: 'src/a.ts', quote: '  const x = 1\nconst y = 2' }),
    'a.ts · const x = 1'
  )
  assert.equal(
    chipText({
      path: 'src/a.ts',
      quote: '<<<<<<< ours\nLOW = 10\n=======\nLOW = 3\n>>>>>>> theirs'
    }),
    'a.ts · LOW = 10'
  )
  assert.equal(chipText({ path: 'a.ts', quote: '' }), 'a.ts')
  assert.equal(chipText({ path: 'a.ts', quote: 'x'.repeat(60) }).length, 'a.ts · '.length + 48)
})

test('the author shows once per run, and a submit ends the run', () => {
  const human = (id: number): WorkgroupThreadMessage =>
    message(id, 'message', { author: 'human', tabId: '' })
  const messages = [
    message(1, 'note'),
    message(2, 'note'),
    human(3),
    message(4, 'submit'),
    message(5, 'message')
  ]
  assert.deepEqual(
    messages.map((_, i) => startsRun(messages, i)),
    [true, false, true, true, true]
  )
})

test('a reply attachment shows the first non-empty line, shortened', () => {
  assert.equal(firstLine('\n  Which threshold?\nmore'), 'Which threshold?')
  assert.equal(firstLine('y'.repeat(100), 10), `${'y'.repeat(9)}…`)
})
