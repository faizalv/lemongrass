// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  cacheLeftMs,
  cacheTtlKnown,
  closeReminderDue,
  isColdAndLarge
} from '../preload/cacheState.ts'

const at = (seconds: number): number => seconds * 1000

test('the cache lives for the recorded lifetime, or five minutes when unknown', () => {
  assert.equal(
    cacheLeftMs({ used_tokens: 1, window_tokens: 1, seen_at: 100, cache_ttl: 3600 }, at(200)),
    at(3500)
  )
  assert.equal(cacheLeftMs({ used_tokens: 1, window_tokens: 1, seen_at: 100 }, at(400)), 0)
})

test('only a large context past its lifetime is cold and large', () => {
  const big = { used_tokens: 400_000, window_tokens: 1_000_000, seen_at: 0, cache_ttl: 3600 }
  assert.equal(isColdAndLarge(big, at(3599)), false)
  assert.equal(isColdAndLarge(big, at(3601)), true)
  assert.equal(isColdAndLarge({ ...big, used_tokens: 90_000 }, at(9999)), true)
  assert.equal(isColdAndLarge({ ...big, used_tokens: 20_000 }, at(9999)), false)
  assert.equal(isColdAndLarge(undefined, at(9999)), false)
})

test('a lifetime is known only when the transcript gave one', () => {
  assert.equal(
    cacheTtlKnown({ used_tokens: 1, window_tokens: 1, seen_at: 0, cache_ttl: 3600 }),
    true
  )
  assert.equal(cacheTtlKnown({ used_tokens: 1, window_tokens: 1, seen_at: 0 }), false)
})

test('the close reminder waits half the lifetime, ten minutes at most, and ends with the cache', () => {
  const hour = { used_tokens: 1, window_tokens: 1, seen_at: 0, cache_ttl: 3600 }
  assert.equal(closeReminderDue(hour, at(9 * 60)), false)
  assert.equal(closeReminderDue(hour, at(10 * 60)), true)
  assert.equal(closeReminderDue(hour, at(3600)), false)
  const fiveMinutes = { used_tokens: 1, window_tokens: 1, seen_at: 0 }
  assert.equal(closeReminderDue(fiveMinutes, at(149)), false)
  assert.equal(closeReminderDue(fiveMinutes, at(150)), true)
})
