// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { composeReview } from './reviewCompose.ts'
import type { Annotation, ConflictAnchor } from '../preload/types.ts'

function anchor(fields: Partial<ConflictAnchor> = {}): ConflictAnchor {
  return {
    kind: 'conflict',
    path: 'src/app.ts',
    startLine: 12,
    endLine: 18,
    pane: 'middle',
    operation: 'rebase',
    leftRef: 'feat',
    rightRef: 'origin/main',
    ...fields
  }
}

function note(
  id: string,
  fields: Partial<ConflictAnchor>,
  quote: string,
  comment: string
): Annotation {
  return { id, anchor: anchor(fields), quote, comment }
}

test('a conflict review names the operation, then each comment with its place and text', () => {
  const text = composeReview({
    message: 'Keep both sides where you can.',
    annotations: [
      note('a', {}, 'const a = 1\nconst b = 2\n', 'Rename b to total.'),
      note('b', { path: 'README.md', pane: 'left', startLine: 3, endLine: 3 }, 'Old intro', 'Why?')
    ]
  })
  assert.equal(
    text,
    [
      '[lg] The human reviewed the conflicts in the rebase of feat onto origin/main and left ' +
        'these comments for you. Line numbers are of each file as it is on disk, conflict ' +
        'markers included while it is unresolved.',
      'Keep both sides where you can.',
      '1. src/app.ts, lines 12 to 18, middle pane (the result):\n> const a = 1\n> const b = 2\nRename b to total.',
      '2. README.md, line 3, left pane (feat):\n> Old intro\nWhy?'
    ].join('\n\n')
  )
})

test('a merge and a stash read as such, and the right pane names the right ref', () => {
  const merge = composeReview({
    message: '',
    annotations: [
      note('a', { operation: 'merge', leftRef: 'main', rightRef: 'feat', pane: 'right' }, 'x', 'y')
    ]
  })
  assert.ok(
    merge.startsWith('[lg] The human reviewed the conflicts in the merge of feat into main ')
  )
  assert.ok(merge.includes('right pane (feat)'))
  const stash = composeReview({
    message: '',
    annotations: [
      note('a', { operation: 'stash', leftRef: 'stashed changes', rightRef: 'main' }, 'x', 'y')
    ]
  })
  assert.ok(stash.includes('the conflicts in the stashed changes applied onto main '))
})

test("blank comments are dropped, and a message alone is still marked as the human's", () => {
  assert.equal(composeReview({ message: '  ', annotations: [note('a', {}, 'x', '  ')] }), '')
  assert.equal(
    composeReview({ message: 'Just this.', annotations: [] }),
    '[lg] The human sent you this review.\n\nJust this.'
  )
})
