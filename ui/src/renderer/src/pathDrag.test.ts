// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { quotePath } from './pathDrag.ts'

test('a plain path is pasted as it is', () => {
  assert.equal(
    quotePath('biblio/scratchpad/some-task/prd.md'),
    'biblio/scratchpad/some-task/prd.md'
  )
})

test('a path a shell would split or expand is single-quoted', () => {
  assert.equal(quotePath('docs/my notes.md'), "'docs/my notes.md'")
  assert.equal(quotePath('src/$(x).ts'), "'src/$(x).ts'")
  assert.equal(quotePath("it's.md"), "'it'\\''s.md'")
})
