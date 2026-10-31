// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  composeReview,
  composeShow,
  composeStartLine,
  type ConflictShow,
  type ReviewDetails
} from './reviewCompose.ts'
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

function show(fields: Partial<ConflictShow> = {}): ConflictShow {
  return {
    operation: 'merge',
    leftRef: 'main',
    rightRef: 'feature',
    threadId: 'abc123',
    files: [
      {
        path: 'src/tax.js',
        unresolved: [{ start: 4, end: 8 }],
        resolved: [{ start: 12, end: 12 }]
      }
    ],
    intent: {
      left: {
        commits: [{ hash: 'a1b2c3d', subject: 'Raise the tax rate', body: '' }],
        more: 0,
        missing: null
      },
      right: {
        commits: [
          {
            hash: 'e4f5a6b',
            subject: 'Rename TAX_RATE to TAX',
            body: 'Every caller uses TAX now.\n\nThe old name is gone.'
          }
        ],
        more: 3,
        missing: null
      }
    },
    ...fields
  }
}

test('show names the conflict, the thread, each file and what each side meant', () => {
  assert.equal(
    composeShow(show()),
    [
      'The human handed you the conflicts in the merge of feature into main. Line numbers are of each file as it is on disk now.',
      '',
      'Conflict thread: abc123',
      '',
      'Files:',
      'src/tax.js',
      '  still in conflict: lines 4-8',
      '  resolved by the human, keep as is: line 12',
      '',
      'Intent of main, the current branch, commits that touched these files since the merge base:',
      '- a1b2c3d Raise the tax rate',
      '',
      'Intent of feature, the branch being merged, commits that touched these files since the merge base:',
      '- e4f5a6b Rename TAX_RATE to TAX',
      '    Every caller uses TAX now.',
      '',
      '    The old name is gone.',
      '  and 3 more'
    ].join('\n')
  )
})

test('show says which side is which in a rebase and a stash apply', () => {
  const rebase = composeShow(show({ operation: 'rebase', leftRef: 'feature', rightRef: 'main' }))
  assert.match(rebase, /in the rebase of feature onto main\./)
  assert.match(
    rebase,
    /Intent of feature, the branch being rebased, the commit being replayed now:/
  )
  assert.match(
    rebase,
    /Intent of main, the upstream, commits that touched these files since feature branched off, the commits of feature already replayed included:/
  )

  const stash = composeShow(
    show({ operation: 'stash', leftRef: 'stashed changes', rightRef: 'main' })
  )
  assert.match(stash, /in the stashed changes applied onto main\./)
  assert.match(
    stash,
    /Intent of the stashed changes, the newest stash, which is most likely the one applied:/
  )
  assert.match(
    stash,
    /Intent of main, the current branch, commits that touched these files since the stash was made:/
  )
})

test('show says when a side has no commits or git could not tell', () => {
  const text = composeShow(
    show({
      intent: {
        left: { commits: [], more: 0, missing: null },
        right: { commits: [], more: 0, missing: 'git has no merge base for this merge' }
      }
    })
  )
  assert.match(text, /since the merge base:\n {2}none/)
  assert.match(text, /since the merge base:\n {2}unknown, git has no merge base for this merge$/)
})

test('the start line is one [lg] line naming the operation and the manual', () => {
  assert.equal(
    composeStartLine({ operation: 'merge', leftRef: 'main', rightRef: 'feature' }),
    '[lg] the human handed you the conflicts in the merge of feature into main. Run lgrass how git resolve.'
  )
  assert.equal(
    composeStartLine({ operation: 'rebase', leftRef: 'feature', rightRef: 'origin/main' }),
    '[lg] the human handed you the conflicts in the rebase of feature onto origin/main. Run lgrass how git resolve.'
  )
  assert.equal(
    composeStartLine({ operation: 'stash', leftRef: 'stashed changes', rightRef: 'main' }),
    '[lg] the human handed you the conflicts in the stashed changes applied onto main. Run lgrass how git resolve.'
  )
})
