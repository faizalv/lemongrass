// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  addAnnotation,
  clearReview,
  clearScopes,
  editAnnotation,
  removeAnnotation,
  reviewOf,
  scopesOf,
  setReviewMessage
} from './annotations.ts'
import type { AnnotationAnchor } from '../../preload/types'

const anchor = { kind: 'conflict' } as AnnotationAnchor

test('reading an unknown scope does not create it', () => {
  assert.deepEqual(reviewOf('p0', 'x').annotations, [])
  assert.deepEqual(scopesOf('p0'), [])
})

test('add, edit and remove a comment', () => {
  const id = addAnnotation('p1', 's', anchor, 'quoted', 'first')
  assert.equal(reviewOf('p1', 's').annotations.length, 1)
  editAnnotation('p1', 's', id, 'second')
  assert.equal(reviewOf('p1', 's').annotations[0].comment, 'second')
  removeAnnotation('p1', 's', id)
  assert.equal(reviewOf('p1', 's').annotations.length, 0)
})

test('scopes and projects are kept apart', () => {
  addAnnotation('p2', 'a', anchor, 'q', 'c')
  addAnnotation('p2', 'b', anchor, 'q', 'c')
  addAnnotation('p3', 'a', anchor, 'q', 'c')
  setReviewMessage('p2', 'a', 'overall')
  assert.equal(reviewOf('p2', 'a').message, 'overall')
  assert.equal(reviewOf('p2', 'b').message, '')
  clearReview('p2', 'a')
  assert.deepEqual(scopesOf('p2'), ['b'])
  assert.equal(reviewOf('p3', 'a').annotations.length, 1)
})

test('clearScopes drops only the matching prefix', () => {
  addAnnotation('p4', 'conflict:merge:a:b', anchor, 'q', 'c')
  addAnnotation('p4', 'web:page', anchor, 'q', 'c')
  clearScopes('p4', 'conflict:')
  assert.deepEqual(scopesOf('p4'), ['web:page'])
})
