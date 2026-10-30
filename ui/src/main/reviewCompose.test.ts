// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { composeReview, type ReviewDetails } from './reviewCompose.ts'
import type { Annotation, ConflictAnchor, ConflictReviewScope } from '../preload/types.ts'

const SCOPE: ConflictReviewScope = {
  kind: 'conflict',
  operation: 'merge',
  leftRef: 'main',
  rightRef: 'feature',
  files: []
}

const NO_FILES: ReviewDetails = { kind: 'conflict', files: [] }

function anchor(fields: Partial<ConflictAnchor> = {}): ConflictAnchor {
  return {
    kind: 'conflict',
    path: 'src/app.ts',
    startLine: 12,
    endLine: 18,
    pane: 'middle',
    operation: 'merge',
    leftRef: 'main',
    rightRef: 'feature',
    ...fields
  }
}

function note(fields: Partial<ConflictAnchor>, quote: string, comment: string): Annotation {
  return { id: 'a', anchor: anchor(fields), quote, comment }
}

test('a review has the scope heading, the overall comment, each file and each comment', () => {
  const text = composeReview(
    {
      scope: SCOPE,
      message: 'I fixed one block, do the rest.',
      annotations: [
        note({}, 'const a = 1\nconst b = 2\n', 'Rename b to total.'),
        note({ path: 'README.md', pane: 'left', startLine: 3, endLine: 3 }, 'Old intro', 'Why?')
      ]
    },
    {
      kind: 'conflict',
      files: [
        {
          path: 'src/inventory.js',
          unresolved: [
            { start: 4, end: 8 },
            { start: 17, end: 30 }
          ],
          resolved: [{ start: 10, end: 14 }]
        },
        { path: 'src/done.js', unresolved: [], resolved: [{ start: 2, end: 2 }] },
        { path: 'logo.png', unresolved: null, resolved: [] }
      ]
    }
  )
  assert.equal(
    text,
    [
      '[lg] The human reviewed the conflicts in the merge of feature into main and left this ' +
        'review for you. Line numbers are of each file as it is on disk now.',
      'Overall comment: I fixed one block, do the rest.',
      [
        'Files:',
        'src/inventory.js',
        '  still in conflict: lines 4-8, 17-30',
        '  resolved by the human, keep as is: lines 10-14',
        'src/done.js',
        '  no conflict markers left, not staged yet',
        '  resolved by the human, keep as is: line 2',
        'logo.png',
        '  still in conflict: no line markers (binary, or deleted on one side), keep one side whole'
      ].join('\n'),
      'Comments:\n1. src/app.ts, lines 12-18, middle pane (the result):\n> const a = 1\n> const b = 2\n' +
        'Rename b to total.\n\n2. README.md, line 3, left pane (main):\n> Old intro\nWhy?'
    ].join('\n\n')
  )
})

test('an overall comment alone still names the operation, as rebase and stash read', () => {
  const rebase = composeReview(
    {
      scope: { ...SCOPE, operation: 'rebase', leftRef: 'feat', rightRef: 'origin/main' },
      message: 'Do the rest.',
      annotations: []
    },
    NO_FILES
  )
  assert.equal(
    rebase,
    '[lg] The human reviewed the conflicts in the rebase of feat onto origin/main and left this ' +
      'review for you. Line numbers are of each file as it is on disk now.\n\nOverall comment: Do the rest.'
  )
  const stash = composeReview(
    {
      scope: { ...SCOPE, operation: 'stash', leftRef: 'stashed changes', rightRef: 'main' },
      message: 'x',
      annotations: []
    },
    NO_FILES
  )
  assert.ok(stash.includes('the conflicts in the stashed changes applied onto main '))
})

test('nothing is sent without an overall comment or a block comment', () => {
  assert.equal(
    composeReview({ scope: SCOPE, message: '  ', annotations: [note({}, 'x', '  ')] }, NO_FILES),
    ''
  )
})
