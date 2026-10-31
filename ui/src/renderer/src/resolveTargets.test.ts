// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { orderTargets } from './resolveTargets.ts'

const shell = (
  id: string,
  command = 'claude'
): { id: string; kind: string; command: string; label: string } => ({
  id,
  kind: 'shell',
  command,
  label: `label ${id}`
})

test('the tab each pane shows comes first, in pane order, then the rest in layout order', () => {
  const leaves = [
    { type: 'leaf' as const, id: 'p1', tabs: [shell('a'), shell('b')], activeTabId: 'b' },
    { type: 'leaf' as const, id: 'p2', tabs: [shell('c'), shell('d', 'codex')], activeTabId: 'd' }
  ]
  const order = orderTargets(leaves, { a: 'Claude Code 1' })
  assert.deepEqual(
    order.map((t) => [t.tabId, t.paneId, t.visible]),
    [
      ['b', 'p1', true],
      ['d', 'p2', true],
      ['a', 'p1', false],
      ['c', 'p2', false]
    ]
  )
  assert.equal(order.find((t) => t.tabId === 'a')?.label, 'Claude Code 1')
  assert.equal(order.find((t) => t.tabId === 'c')?.label, 'label c')
})

test('plain shells and other tab kinds are never offered', () => {
  const leaves = [
    {
      type: 'leaf' as const,
      id: 'p1',
      tabs: [shell('s', 'bash'), { id: 'doc', kind: 'doc' }, shell('a')],
      activeTabId: 'doc'
    }
  ]
  assert.deepEqual(
    orderTargets(leaves, {}).map((t) => [t.tabId, t.visible]),
    [['a', false]]
  )
})
