// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { apiUsers, buildCall } from './resterClientCall.ts'
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
  ]
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
