// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import type { WorkgroupThreadMessage } from '../../preload/types'
import { mergeMessages } from './conflictThreadModel.ts'

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
    data: null,
    ...extra
  }
}

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
