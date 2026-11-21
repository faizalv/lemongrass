// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { planStatus } from './planMarks.ts'

test('every Plans item checked is done', () => {
  const prd =
    '# T\n\n## Plans\n\n- [x] 1. one\n  - [x] sub\n- \\[x\\] 2. two\n\n## Open\n\n- [ ] later\n'
  assert.equal(planStatus(prd), 'done')
})

test('an unchecked Plans item, plain or escaped, is open', () => {
  assert.equal(planStatus('## Plans\n- [x] a\n  - [ ] b\n'), 'open')
  assert.equal(planStatus('## Plans\n- [x] a\n- \\[ \\] b\n'), 'open')
})

test('only the user approval item open waits for approval', () => {
  assert.equal(planStatus('## Plans\n- [x] a\n- [ ] User approval\n'), 'approval')
  assert.equal(planStatus('## Plans\n- [ ] a\n- [ ] User approval\n'), 'open')
  assert.equal(planStatus('## Plans\n- [x] a\n- [x] User approval\n'), 'done')
})

test('no checkbox at all is open', () => {
  assert.equal(planStatus('## Plans\nJust prose.\n'), 'open')
})

test('a PRD without a Plans heading is read whole', () => {
  assert.equal(planStatus('- [x] a\n- [x] b\n'), 'done')
})
