// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { dropDraft, saveDraft, takeDraft, type ConflictDraft } from './conflictDrafts.ts'

const draft: ConflictDraft = {
  stages: 'stages-a',
  chunks: [],
  finalNewline: true,
  loadedText: '',
  refs: { operation: 'merge', leftRef: 'main', rightRef: 'feat' },
  middle: null,
  workingSeen: null,
  focus: true,
  sizes: [1, 2, 1],
  current: 1
}

test('a draft comes back once, for the same stages only', () => {
  saveDraft('p', 'a.txt', draft)
  assert.equal(takeDraft('p', 'b.txt', 'stages-a'), null)
  assert.deepEqual(takeDraft('p', 'a.txt', 'stages-a'), draft)
  assert.equal(takeDraft('p', 'a.txt', 'stages-a'), null)
  saveDraft('p', 'a.txt', draft)
  assert.equal(takeDraft('p', 'a.txt', 'stages-b'), null)
  saveDraft('p', 'a.txt', draft)
  dropDraft('p', 'a.txt')
  assert.equal(takeDraft('p', 'a.txt', 'stages-a'), null)
})
