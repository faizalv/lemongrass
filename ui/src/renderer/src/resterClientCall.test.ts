// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { apiUsers, buildCall, withBodyDefaults } from './resterClientCall.ts'
import type { VaultResterUser } from '../../preload/types'

const request = {
  id: '1',
  name: 'orders',
  rester: 'shop',
  environment: 'staging',
  user: 'alice',
  method: 'POST',
  url: ' /orders ',
  headers: [
    { name: ' X-Trace ', value: '1' },
    { name: '', value: 'ignored' }
  ],
  bodyKind: 'form' as const,
  json: '{"a":1}',
  form: [
    { name: 'q', value: 'a b' },
    { name: ' ', value: 'x' }
  ],
  multipart: [
    { name: ' doc ', kind: 'file' as const, value: '', path: '/d/a.pdf', size: 3, contentType: '' },
    { name: '', kind: 'text' as const, value: 'skipped', path: '', size: 0, contentType: '' }
  ],
  file: { path: '/d/b.csv', size: 4, contentType: 'text/csv' }
}

test('a form body is urlencoded with its content type', () => {
  const call = buildCall(request)
  assert.equal(call.path, '/orders')
  assert.equal(call.body, 'q=a+b')
  assert.equal(call.contentType, 'application/x-www-form-urlencoded')
  assert.deepEqual(call.headers, [{ name: 'X-Trace', value: '1' }])
})

test('a json body leaves the content type to the proxy, and none sends nothing', () => {
  assert.deepEqual(
    [
      buildCall({ ...request, bodyKind: 'json' }).body,
      buildCall({ ...request, bodyKind: 'json' }).contentType
    ],
    ['{"a":1}', '']
  )
  assert.equal(buildCall({ ...request, bodyKind: 'none' }).body, '')
})

test('an empty user under a rester is sent as no user, and Plain never is', () => {
  assert.equal(buildCall(request).noUser, undefined)
  assert.equal(buildCall({ ...request, user: '' }).noUser, true)
  assert.equal(buildCall({ ...request, rester: '', environment: '', user: '' }).noUser, undefined)
})

test('apiUsers keeps assigned users that can log in or carry a token', () => {
  const user = (Name: string, envs: string[], Token = ''): VaultResterUser => ({
    Name,
    Fields: null,
    FieldTypes: null,
    Token,
    Tags: null,
    Environments: envs,
    Overrides: null
  })
  const rester = {
    API: {
      LoginEndpoint: '',
      TokenPath: '',
      TTLOrigin: '',
      FixedTTLSeconds: 0,
      TokenPlacement: { Kind: 'header' as const, Name: 'Authorization', Prefix: '' },
      Headers: null
    },
    Web: null,
    Environments: [{ Name: 'staging', BaseURL: 'https://x.test', Origin: '', Headers: null }],
    Users: [
      user('login', ['staging']),
      user('token', ['staging'], 't'),
      user('elsewhere', ['prod'], 't')
    ]
  }
  assert.deepEqual(
    apiUsers(rester, 'staging').map((u) => u.Name),
    ['token']
  )
  rester.API.LoginEndpoint = '/login'
  assert.deepEqual(
    apiUsers(rester, 'staging').map((u) => u.Name),
    ['login', 'token']
  )
})

test('a multipart body travels as named parts and a file body as its path', () => {
  const multipart = buildCall({ ...request, bodyKind: 'multipart' })
  assert.equal(multipart.body, '')
  assert.equal(multipart.contentType, '')
  assert.deepEqual(multipart.multipart, [
    { name: 'doc', kind: 'file', value: '', path: '/d/a.pdf', size: 3, contentType: '' }
  ])
  assert.equal(multipart.file, undefined)
  const file = buildCall({ ...request, bodyKind: 'file' })
  assert.deepEqual(file.file, { path: '/d/b.csv', size: 4, contentType: 'text/csv' })
  assert.equal(file.multipart, undefined)
  assert.equal(buildCall({ ...request, bodyKind: 'form' }).multipart, undefined)
})

test('a multipart body with no named part sends nothing', () => {
  const call = buildCall({ ...request, bodyKind: 'multipart', multipart: [] })
  assert.equal(call.multipart, undefined)
  assert.equal(call.body, '')
})

test('an older saved request gets empty multipart and file fields', () => {
  const old = { ...request } as Partial<typeof request>
  delete old.multipart
  delete old.file
  const filled = withBodyDefaults(old as typeof request)
  assert.deepEqual(filled.multipart, [])
  assert.deepEqual(filled.file, { path: '', size: 0, contentType: '' })
})
