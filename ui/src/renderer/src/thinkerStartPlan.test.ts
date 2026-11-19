// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { ThinkerHolds, planThinkerStart, thinkerCacheKey } from './thinkerStartPlan.ts'

const opus = thinkerCacheKey('claude', 'work', 'opus')
const haiku = thinkerCacheKey('claude', 'work', 'haiku')

test('the cache key separates vendor, Claude account and model, and Antigravity has none', () => {
  assert.notEqual(
    thinkerCacheKey('claude', 'work', 'opus'),
    thinkerCacheKey('claude', 'home', 'opus')
  )
  assert.notEqual(opus, haiku)
  assert.notEqual(thinkerCacheKey('claude', '', ''), thinkerCacheKey('codex', '', ''))
  assert.equal(thinkerCacheKey('codex', 'work', 'gpt'), thinkerCacheKey('codex', 'home', 'gpt'))
  assert.equal(thinkerCacheKey('agy', 'work', 'x'), null)
})

test('the first thinker of each key starts and the rest wait for it', () => {
  const plan = planThinkerStart(
    [
      { tabId: 'a', key: opus },
      { tabId: 'b', key: opus },
      { tabId: 'c', key: haiku },
      { tabId: 'd', key: opus },
      { tabId: 'e', key: haiku }
    ],
    new Set()
  )
  assert.deepEqual(plan.start, ['a', 'c'])
  assert.deepEqual(
    [...plan.held],
    [
      ['a', ['b', 'd']],
      ['c', ['e']]
    ]
  )
})

test('a lone thinker, an Antigravity thinker and a warm key start at once', () => {
  const plan = planThinkerStart(
    [
      { tabId: 'a', key: opus },
      { tabId: 'b', key: null },
      { tabId: 'c', key: null },
      { tabId: 'd', key: haiku },
      { tabId: 'e', key: haiku }
    ],
    new Set([haiku!])
  )
  assert.deepEqual(plan.start, ['a', 'b', 'c', 'd', 'e'])
  assert.equal(plan.held.size, 0)
})

function holdsWithFakeClock(): {
  holds: ThinkerHolds
  started: string[]
  fire: () => void
  pending: () => number
} {
  const started: string[] = []
  const timers = new Map<number, () => void>()
  let next = 0
  const holds = new ThinkerHolds(
    (tabId) => started.push(tabId),
    {
      set: (fn) => {
        timers.set(++next, fn)
        return next
      },
      clear: (handle) => timers.delete(handle as number)
    },
    30_000
  )
  const fire = (): void => {
    for (const [id, fn] of [...timers]) {
      timers.delete(id)
      fn()
    }
  }
  return { holds, started, fire, pending: () => timers.size }
}

const trio = [
  { tabId: 'a', key: opus },
  { tabId: 'b', key: opus },
  { tabId: 'c', key: opus }
]

test('the first thinker starting releases its held tabs once and cancels the timeout', () => {
  const { holds, started, pending } = holdsWithFakeClock()
  holds.plan('/p', trio)
  assert.ok(holds.isHeld('b'))
  holds.noteStarted('b')
  assert.deepEqual(started, [])
  holds.noteStarted('a')
  assert.deepEqual(started, ['b', 'c'])
  assert.equal(pending(), 0)
  holds.noteStarted('a')
  assert.deepEqual(started, ['b', 'c'])
  assert.ok(!holds.isHeld('b'))
})

test('the timeout releases held tabs when the first thinker never starts', () => {
  const { holds, started, fire } = holdsWithFakeClock()
  holds.plan('/p', trio)
  fire()
  assert.deepEqual(started, ['b', 'c'])
})

test('a first thinker that is gone releases its tabs, and a closed held tab is never started', () => {
  const { holds, started } = holdsWithFakeClock()
  holds.plan('/p', trio)
  assert.equal(holds.noteGone('c'), true)
  assert.equal(holds.noteGone('a'), false)
  assert.deepEqual(started, ['b'])
})

test('a key whose thinker started in the same project skips the wait, and only there', () => {
  const { holds } = holdsWithFakeClock()
  holds.plan('/p', [{ tabId: 'a', key: opus }])
  holds.noteStarted('a')
  const pair = (x: string, y: string): { tabId: string; key: string | null }[] => [
    { tabId: x, key: opus },
    { tabId: y, key: opus }
  ]
  assert.deepEqual(holds.plan('/p', pair('d', 'e')).start, ['d', 'e'])
  assert.deepEqual(holds.plan('/other', pair('f', 'g')).start, ['f'])
  holds.noteGone('a')
  holds.noteGone('d')
  holds.noteGone('e')
  assert.deepEqual(holds.plan('/p', pair('h', 'i')).start, ['h'])
})
