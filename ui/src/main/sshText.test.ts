// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  fingerprint,
  hostBlockRange,
  knownHostsLookup,
  parseKnownHostsMatches,
  parsePublicKeyLine
} from './sshText.ts'

// A made-up ed25519 public key; only its encoding matters here.
const BLOB = 'AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl'

test('a public key line splits into type, blob and comment', () => {
  assert.deepEqual(parsePublicKeyLine(`ssh-ed25519 ${BLOB} me@laptop extra\n`), {
    type: 'ssh-ed25519',
    blob: BLOB,
    comment: 'me@laptop extra'
  })
  assert.equal(parsePublicKeyLine('not a key'), null)
})

test('the fingerprint is unpadded base64 of the blob digest, as ssh-keygen prints it', () => {
  assert.equal(fingerprint(BLOB), 'SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU')
})

test('known_hosts lookups use the bracket form off port 22 and prefer HostKeyAlias', () => {
  const base = {
    hostname: 'h.example',
    user: '',
    port: '22',
    proxyJump: '',
    identityFiles: [],
    knownHostsFiles: [],
    hostKeyAlias: ''
  }
  assert.equal(knownHostsLookup(base), 'h.example')
  assert.equal(knownHostsLookup({ ...base, port: '2222' }), '[h.example]:2222')
  assert.equal(knownHostsLookup({ ...base, hostKeyAlias: 'box' }), 'box')
})

test('ssh-keygen -F output yields one key per match, markers and hashed names included', () => {
  const out = [
    '# Host h.example found: line 3',
    `h.example ssh-ed25519 ${BLOB}`,
    `|1|abc=|def= ssh-ed25519 ${BLOB}`,
    `@cert-authority *.example ssh-ed25519 ${BLOB}`
  ].join('\n')
  const keys = parseKnownHostsMatches(out, '/k')
  assert.equal(keys.length, 3)
  assert.equal(keys[0].type, 'ssh-ed25519')
  assert.equal(keys[0].fingerprint, fingerprint(BLOB))
})

test('a host block runs from its Host line to the next Host or Match', () => {
  const text = [
    'Host *',
    '  ServerAliveInterval 30',
    '',
    'Host a b',
    '  HostName h',
    '',
    'Match host x',
    'Host c'
  ].join('\n')
  assert.deepEqual(hostBlockRange(text, 'b'), { start: 4, end: 5 })
  assert.deepEqual(hostBlockRange(text, 'c'), { start: 8, end: 8 })
  assert.equal(hostBlockRange(text, 'missing'), null)
})
