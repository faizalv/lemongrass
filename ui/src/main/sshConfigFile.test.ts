// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  INCLUDE_LINE,
  includeState,
  parseManagedConfig,
  renderManagedConfig,
  validateHostFields,
  withIncludeLine
} from './sshConfigFile.ts'
import type { SSHHostFields } from '../preload/types'

const home = '/home/u'

function fields(over: Partial<SSHHostFields> = {}): SSHHostFields {
  return {
    alias: 'web-staging',
    hostName: '203.0.113.10',
    user: 'deploy',
    port: '2222',
    identityFile: '/home/u/.ssh/id_ed25519',
    proxyJump: 'bastion',
    identitiesOnly: true,
    ...over
  }
}

test('a managed file round-trips its hosts and keeps lines it does not edit', () => {
  const text = renderManagedConfig(
    { preamble: [], hosts: [{ fields: fields(), extra: ['ServerAliveInterval 30'] }] },
    home
  )
  assert.match(text, /IdentityFile ~\/\.ssh\/id_ed25519/)
  const parsed = parseManagedConfig(text, home)
  assert.equal(parsed.hosts.length, 1)
  assert.deepEqual(parsed.hosts[0].fields, fields())
  assert.deepEqual(parsed.hosts[0].extra, ['ServerAliveInterval 30'])
  assert.equal(renderManagedConfig(parsed, home), text)
})

test('an identity file with a space is quoted and read back', () => {
  const f = fields({ identityFile: '/home/u/.ssh/my key' })
  const text = renderManagedConfig({ preamble: [], hosts: [{ fields: f, extra: [] }] }, home)
  assert.match(text, /IdentityFile "~\/\.ssh\/my key"/)
  assert.equal(parseManagedConfig(text, home).hosts[0].fields.identityFile, f.identityFile)
})

test('host fields that could break the config line are refused', () => {
  assert.equal(validateHostFields(fields()), null)
  assert.notEqual(validateHostFields(fields({ alias: '-oProxyCommand=x' })), null)
  assert.notEqual(validateHostFields(fields({ hostName: 'a b' })), null)
  assert.notEqual(validateHostFields(fields({ hostName: '-x' })), null)
  assert.notEqual(validateHostFields(fields({ user: 'a\nHost x' })), null)
  assert.notEqual(validateHostFields(fields({ port: '70000' })), null)
  assert.notEqual(validateHostFields(fields({ identityFile: 'relative' })), null)
  assert.notEqual(validateHostFields(fields({ proxyJump: 'a b' })), null)
  assert.equal(validateHostFields(fields({ user: '', port: '', proxyJump: '' })), null)
})

test('include state follows where the Include line sits', () => {
  assert.equal(includeState(null, home), 'missing')
  assert.equal(includeState('Host a\n  HostName b\n', home), 'missing')
  assert.equal(includeState(`${INCLUDE_LINE}\n\nHost a\n`, home), 'included')
  assert.equal(includeState('# top\nInclude ~/.ssh/config.d/*\nHost a\n', home), 'included')
  assert.equal(includeState('Include /home/u/.ssh/config.d/lemongrass.conf\n', home), 'included')
  assert.equal(includeState(`Host a\n  ${INCLUDE_LINE}\n`, home), 'insideBlock')
  assert.equal(includeState('Include config.d/other.conf\n', home), 'missing')
})

test('the Include line goes above everything already in the config', () => {
  assert.equal(withIncludeLine(null), `${INCLUDE_LINE}\n\n`)
  const out = withIncludeLine('Host a\n')
  assert.ok(out.startsWith(INCLUDE_LINE))
  assert.equal(includeState(out, home), 'included')
})
