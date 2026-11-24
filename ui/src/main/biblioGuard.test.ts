// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { isBiblioCategory } from './biblioGuard.ts'

test('the category folders and the root are protected, in any spelling', () => {
  for (const path of [
    '',
    '.',
    'books',
    'books/',
    './laws',
    'handover',
    'scratchpad',
    'scratchpad/archive',
    'scratchpad//archive/',
    'scratchpad/task/..',
    'books/../laws'
  ])
    assert.equal(isBiblioCategory(path), true, path)
})

test('entries inside the categories are not protected', () => {
  for (const path of [
    'books/toc.md',
    'scratchpad/task',
    'scratchpad/archive/task',
    'laws/rule.md',
    'handover/whiteboard.md'
  ])
    assert.equal(isBiblioCategory(path), false, path)
})
