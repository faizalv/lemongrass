// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { nextCheckDelay, parseCheckCache } from './updateCheckCache.ts'

const HOUR = 60 * 60 * 1000
const release = {
  version: '1.2.0',
  assetName: 'lemongrass-1.2.0.x86_64.rpm',
  assetUrl: 'https://example.invalid/a.rpm',
  assetSize: 10,
  sumsUrl: 'https://example.invalid/SHA256SUMS'
}

test('parseCheckCache keeps a cache from the same channel', () => {
  const text = JSON.stringify({ checkedAt: 1000, prerelease: false, version: '1.2.0', release })
  assert.deepEqual(parseCheckCache(text, false, 2000), {
    checkedAt: 1000,
    prerelease: false,
    version: '1.2.0',
    release
  })
})

test('parseCheckCache keeps a release with no package for this platform', () => {
  const text = JSON.stringify({
    checkedAt: 1000,
    prerelease: true,
    version: '1.2.0',
    release: null
  })
  assert.equal(parseCheckCache(text, true, 2000)?.release, null)
})

test('parseCheckCache drops the other channel, a future check and a bad shape', () => {
  const ok = { checkedAt: 1000, prerelease: false, version: '1.2.0', release }
  assert.equal(parseCheckCache(JSON.stringify(ok), true, 2000), null)
  assert.equal(parseCheckCache(JSON.stringify(ok), false, 500), null)
  assert.equal(
    parseCheckCache(JSON.stringify({ ...ok, release: { version: '1.2.0' } }), false, 2000),
    null
  )
  assert.equal(parseCheckCache(JSON.stringify({ ...ok, checkedAt: 'soon' }), false, 2000), null)
  assert.equal(parseCheckCache('not json', false, 2000), null)
  assert.equal(parseCheckCache('null', false, 2000), null)
})

test('nextCheckDelay checks at once without a cache or with a stale one', () => {
  assert.equal(nextCheckDelay(null, 5 * HOUR, HOUR), 0)
  assert.equal(nextCheckDelay(HOUR, 5 * HOUR, HOUR), 0)
  assert.equal(nextCheckDelay(4 * HOUR, 5 * HOUR, HOUR), 0)
})

test('nextCheckDelay waits for a fresh cache to turn an hour old', () => {
  assert.equal(nextCheckDelay(5 * HOUR - 10 * 60 * 1000, 5 * HOUR, HOUR), 50 * 60 * 1000)
  assert.equal(nextCheckDelay(5 * HOUR, 5 * HOUR, HOUR), HOUR)
})
