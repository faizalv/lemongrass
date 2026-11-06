// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  parseResolvedHost,
  parseSSHConfigHostSources,
  parseSSHConfigHosts,
  type ConfigFiles
} from './sshHosts.ts'

const home = '/home/u'

function fakeFiles(tree: Record<string, string>): ConfigFiles {
  return {
    read: (path) => tree[path] ?? null,
    list: (dir) =>
      Object.keys(tree)
        .filter((p) => p.startsWith(dir + '/') && !p.slice(dir.length + 1).includes('/'))
        .map((p) => p.slice(dir.length + 1))
  }
}

test('concrete hosts are listed in file order and patterns are skipped', () => {
  const files = fakeFiles({
    '/home/u/.ssh/config': [
      '# comment',
      'Host *',
      '  ServerAliveInterval 30',
      'Host bastion-prod bastion-staging',
      '  HostName 10.0.0.5',
      'Host *.internal !secret db-?',
      'host=jump-1',
      'Match host foo',
      'Host bastion-prod'
    ].join('\n')
  })
  assert.deepEqual(parseSSHConfigHosts('/home/u/.ssh/config', home, files), [
    'bastion-prod',
    'bastion-staging',
    'jump-1'
  ])
})

test('Include is followed relative to the ssh directory, with a glob and a tilde', () => {
  const files = fakeFiles({
    '/home/u/.ssh/config': 'Include config.d/*.conf ~/extra/hosts "/etc/ssh/team"\nHost local-a',
    '/home/u/.ssh/config.d/b.conf': 'Host from-b',
    '/home/u/.ssh/config.d/a.conf': 'Host from-a',
    '/home/u/.ssh/config.d/notes.txt': 'Host ignored',
    '/home/u/extra/hosts': 'Host from-extra',
    '/etc/ssh/team': 'Host from-team'
  })
  assert.deepEqual(parseSSHConfigHosts('/home/u/.ssh/config', home, files), [
    'from-a',
    'from-b',
    'from-extra',
    'from-team',
    'local-a'
  ])
})

test('a missing config or an include loop gives what can be read', () => {
  assert.deepEqual(parseSSHConfigHosts('/home/u/.ssh/config', home, fakeFiles({})), [])
  const loop = fakeFiles({
    '/home/u/.ssh/config': 'Include config\nHost only-once'
  })
  assert.deepEqual(parseSSHConfigHosts('/home/u/.ssh/config', home, loop), ['only-once'])
})

test('ssh -G output gives the resolved target', () => {
  const output = 'host bastion-prod\nuser dev\nhostname 10.0.0.5\nport 2222\nproxyjump jump-1\n'
  assert.deepEqual(parseResolvedHost(output), {
    hostname: '10.0.0.5',
    user: 'dev',
    port: '2222',
    proxyJump: 'jump-1',
    identityFiles: [],
    knownHostsFiles: [],
    hostKeyAlias: ''
  })
  assert.equal(parseResolvedHost('hostname h\nproxyjump none\n').proxyJump, '')
})

test('ssh -G lists every identity file and splits the known hosts files', () => {
  const output = [
    'hostname h',
    'identityfile ~/.ssh/id_work',
    'identityfile ~/.ssh/id_ed25519',
    'userknownhostsfile ~/.ssh/known_hosts ~/.ssh/known_hosts2',
    'hostkeyalias none'
  ].join('\n')
  const resolved = parseResolvedHost(output)
  assert.deepEqual(resolved.identityFiles, ['~/.ssh/id_work', '~/.ssh/id_ed25519'])
  assert.deepEqual(resolved.knownHostsFiles, ['~/.ssh/known_hosts', '~/.ssh/known_hosts2'])
  assert.equal(resolved.hostKeyAlias, '')
})

test('host sources name the file each host first appears in', () => {
  const files = fakeFiles({
    '/home/u/.ssh/config': 'Include config.d/*\nHost a\nHost b',
    '/home/u/.ssh/config.d/lemongrass.conf': 'Host b\nHost c'
  })
  assert.deepEqual(parseSSHConfigHostSources('/home/u/.ssh/config', home, files), [
    { alias: 'b', file: '/home/u/.ssh/config.d/lemongrass.conf' },
    { alias: 'c', file: '/home/u/.ssh/config.d/lemongrass.conf' },
    { alias: 'a', file: '/home/u/.ssh/config' }
  ])
})
