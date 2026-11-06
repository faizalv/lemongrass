// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { crumbsOf, joinRemote, nameProblem, parentOf } from './components/ssh/files/remotePath.ts'

test('joinRemote joins under root and under a folder', () => {
  assert.equal(joinRemote('/', 'etc'), '/etc')
  assert.equal(joinRemote('/srv/app', 'logs'), '/srv/app/logs')
  assert.equal(joinRemote('/srv/app/', 'logs'), '/srv/app/logs')
})

test('parentOf stops at root', () => {
  assert.equal(parentOf('/srv/app'), '/srv')
  assert.equal(parentOf('/srv'), '/')
  assert.equal(parentOf('/'), '/')
})

test('crumbsOf lists root and every folder down to the path', () => {
  assert.deepEqual(crumbsOf('/'), [{ name: '/', path: '/' }])
  assert.deepEqual(crumbsOf('/home/u'), [
    { name: '/', path: '/' },
    { name: 'home', path: '/home' },
    { name: 'u', path: '/home/u' }
  ])
})

test('nameProblem refuses empty, dot names and slashes', () => {
  assert.equal(nameProblem('notes.txt'), null)
  assert.ok(nameProblem(''))
  assert.ok(nameProblem('..'))
  assert.ok(nameProblem('a/b'))
})
