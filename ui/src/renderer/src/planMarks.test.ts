// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { plansAllChecked } from './planMarks.ts'

test('every Plans item checked', () => {
  const prd =
    '# T\n\n## Plans\n\n- [x] 1. one\n  - [x] sub\n- \\[x\\] 2. two\n\n## Open\n\n- [ ] later\n'
  assert.equal(plansAllChecked(prd), true)
})

test('an unchecked Plans item, plain or escaped, is not done', () => {
  assert.equal(plansAllChecked('## Plans\n- [x] a\n  - [ ] b\n'), false)
  assert.equal(plansAllChecked('## Plans\n- [x] a\n- \\[ \\] b\n'), false)
})

test('no checkbox at all is not done', () => {
  assert.equal(plansAllChecked('## Plans\nJust prose.\n'), false)
})

test('a PRD without a Plans heading is read whole', () => {
  assert.equal(plansAllChecked('- [x] a\n- [x] b\n'), true)
})
