// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { apiFacts, appSteps, loginSteps } from './authFacts.ts'
import type { VaultResterAPI } from '../../preload/types'

const api: VaultResterAPI = {
  LoginEndpoint: '/api/login',
  TokenPath: 'data.token',
  TTLOrigin: '',
  FixedTTLSeconds: 3600,
  TokenPlacement: { Kind: 'header', Name: 'Authorization', Prefix: 'Bearer ' },
  Headers: null
}

test('an API side with a login shows its endpoint, token path, lifetime and placement', () => {
  assert.deepEqual(apiFacts(api), [
    { label: 'Login', value: '/api/login', code: true },
    { label: 'Token path', value: 'data.token', code: true },
    { label: 'Lifetime', value: 'Fixed, 60 min', code: false },
    { label: 'Token goes in', prefix: 'Header', value: 'Authorization: Bearer <token>', code: true }
  ])
})

test('an API side with no login endpoint hides the token path and lifetime', () => {
  const facts = apiFacts({ ...api, LoginEndpoint: '', Headers: [{ Name: 'a', Value: 'b' }] })
  assert.deepEqual(
    facts.map((f) => f.label),
    ['Login', 'Token goes in', 'Headers']
  )
  assert.equal(facts[0].code, false)
})

test('the lifetime follows the stored origin', () => {
  assert.equal(apiFacts({ ...api, TTLOrigin: 'jwt-exp' })[2].value, 'Read from the token (JWT exp)')
  assert.deepEqual(apiFacts({ ...api, TTLOrigin: 'expires_in' })[2], {
    label: 'Lifetime',
    prefix: 'From response field',
    value: 'expires_in',
    code: true
  })
})

test('a cookie or query token names where it goes', () => {
  const cookie = apiFacts({ ...api, TokenPlacement: { Kind: 'cookie', Name: 'sid', Prefix: '' } })
  assert.equal(cookie[3].prefix, 'Cookie')
  assert.equal(cookie[3].value, 'sid')
})

test('login steps keep their order, target and the field a fill types', () => {
  assert.deepEqual(
    loginSteps([
      { Action: 'goto', URL: '/login', Selector: '', Field: '', TimeoutSeconds: 0 },
      { Action: 'fill', URL: '', Selector: '#email', Field: 'email', TimeoutSeconds: 0 },
      { Action: 'click', URL: '', Selector: 'button', Field: '', TimeoutSeconds: 0 },
      { Action: 'wait', URL: '**/home', Selector: '', Field: '', TimeoutSeconds: 5 }
    ]),
    [
      { kind: 'goto', label: 'Go to', target: '/login' },
      { kind: 'fill', label: 'Fill', target: '#email', field: 'email' },
      { kind: 'click', label: 'Click', target: 'button' },
      { kind: 'wait', label: 'Wait', target: '**/home' }
    ]
  )
})

test('phone steps name the text or the id they act on', () => {
  assert.deepEqual(
    appSteps([
      { Action: 'launch', Text: '', ID: '', Field: '', Key: '', TimeoutSeconds: 0 },
      { Action: 'tap', Text: 'Sign in', ID: '', Field: '', Key: '', TimeoutSeconds: 0 },
      { Action: 'fill', Text: '', ID: 'email', Field: 'email', Key: '', TimeoutSeconds: 0 },
      { Action: 'key', Text: '', ID: '', Field: '', Key: 'back', TimeoutSeconds: 0 }
    ]),
    [
      { kind: 'launch', label: 'Launch', target: '' },
      { kind: 'tap', label: 'Tap', target: 'Sign in' },
      { kind: 'fill', label: 'Fill', target: 'email', field: 'email' },
      { kind: 'key', label: 'Press', target: 'back' }
    ]
  )
})
