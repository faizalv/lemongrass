// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { STALE_AFTER_MS, reorganizedTabs, staleTabs, type OrderTab } from './paneOrder.ts'

const shell = (id: string): OrderTab => ({ id, kind: 'shell' })
const doc = (id: string): OrderTab => ({ id, kind: 'doc' })
const ids = (tabs: OrderTab[]): string[] => tabs.map((tab) => tab.id)

test('shells come first by latest activity, then other tabs by last seen', () => {
  const tabs = [doc('d1'), shell('s1'), doc('d2'), shell('s2'), shell('s3')]
  const recency = {
    s1: { seenAt: 10, activeAt: 50 },
    s2: { seenAt: 70 },
    s3: { seenAt: 20 },
    d1: { seenAt: 5 },
    d2: { seenAt: 90 }
  }
  assert.deepEqual(ids(reorganizedTabs(tabs, recency)), ['s2', 's1', 's3', 'd2', 'd1'])
})

test('tabs with no recency keep their order after the dated ones', () => {
  const tabs = [doc('a'), doc('b'), doc('c')]
  assert.deepEqual(ids(reorganizedTabs(tabs, { c: { seenAt: 1 } })), ['c', 'a', 'b'])
})

test("a task's tabs follow the most recent shell working it", () => {
  const tabs = [
    shell('s1'),
    shell('s2'),
    shell('s3'),
    doc('p1'),
    doc('p2'),
    doc('free'),
    doc('gone')
  ]
  const recency = {
    s1: { activeAt: 10 },
    s2: { activeAt: 20 },
    s3: { activeAt: 5 },
    p1: { seenAt: 1 },
    p2: { seenAt: 2 }
  }
  const tasks: Record<string, string> = { s1: 'a', s3: 'a', p1: 'a', p2: 'a', gone: 'b' }
  const order = reorganizedTabs(tabs, recency, (tab) => tasks[tab.id] ?? null)
  assert.deepEqual(ids(order), ['s2', 's1', 'p2', 'p1', 's3', 'free', 'gone'])
})

test('a second reorganize changes nothing', () => {
  const tabs = [shell('s1'), doc('a'), doc('b')]
  const once = reorganizedTabs(tabs, {})
  assert.deepEqual(ids(reorganizedTabs(once, {})), ids(once))
})

test('stale tabs are off screen documents older than the limit', () => {
  const now = 10 * STALE_AFTER_MS
  const old = now - STALE_AFTER_MS - 1
  const tabs = [doc('old'), doc('fresh'), doc('active'), shell('s'), doc('unknown')]
  const recency = {
    old: { seenAt: old },
    fresh: { seenAt: now - 1 },
    active: { seenAt: old },
    s: { seenAt: old }
  }
  const stale = staleTabs(tabs, 'active', recency, now, (tab) => tab.kind === 'doc')
  assert.deepEqual(ids(stale), ['old'])
})
