// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import type { DepEntry, DepsReport } from '../../preload/types'
import {
  buttonWeight,
  featuresText,
  listText,
  agentDeps,
  agentGroupText,
  agentMissing,
  anyAgentReady,
  depLabel,
  offPathText,
  originText,
  osText,
  routeBlocked,
  routeNote,
  statusText,
  tone
} from './depsView.ts'

const git: DepEntry = {
  name: 'git',
  label: '',
  required: true,
  agent: false,
  features: ['gitPane', 'rebaseForecast'],
  floor: '2.45.0',
  path: '/usr/bin/git',
  version: '2.43.5',
  origin: { kind: 'package', package: 'git-core' },
  status: 'belowFloor',
  offer: { action: 'upgrade', package: 'git-core', route: 'helper' },
  message: ''
}

const adb: DepEntry = {
  ...git,
  name: 'adb',
  required: false,
  features: ['device', 'newThing'],
  floor: '',
  path: '',
  version: '',
  origin: { kind: 'none' },
  status: 'missing',
  offer: { action: 'install', package: 'android-tools', route: 'helper' }
}

test('buttonWeight follows importance', () => {
  assert.equal(buttonWeight(git), 'primary')
  assert.equal(buttonWeight(adb), 'ghost')
  assert.equal(buttonWeight({ ...adb, offer: null }), null)
})

test('the row texts read as sentences', () => {
  assert.equal(statusText(git), '2.43.5 is older than 2.45.0')
  assert.equal(statusText(adb), 'Not installed')
  assert.equal(originText(git), 'from git-core')
  assert.equal(originText({ ...git, origin: { kind: 'manual' } }), 'installed by hand')
  assert.equal(originText(adb), '')
  assert.equal(featuresText(git.features), 'Used by the Git pane and the rebase forecast.')
  assert.equal(featuresText(adb.features), 'Used by the Device pane and newThing.')
  assert.equal(listText(['a', 'b', 'c']), 'a, b and c')
  assert.equal(osText({ family: 'fedora', id: 'fedora', version: '43' }), 'Fedora 43')
})

test('tone and route', () => {
  assert.equal(tone(git), 'warn')
  assert.equal(tone(adb), 'muted')
  assert.equal(tone({ ...adb, status: 'ok' }), 'ok')
  assert.equal(routeBlocked(git.offer!, null), null)
  assert.equal(routeBlocked({ ...git.offer!, route: 'winget' }, null), null)
  assert.equal(routeBlocked(git.offer!, 'This is a source build.'), 'This is a source build.')
  assert.ok(routeBlocked({ ...git.offer!, route: 'brew' }, null))
  assert.equal(routeNote(git, 'fedora'), null)
  assert.match(
    routeNote(
      { ...git, offer: { ...git.offer!, package: 'Git.Git', route: 'winget' } },
      'windows'
    ) ?? '',
    /license of Git\.Git/
  )
})

const claude: DepEntry = {
  ...git,
  name: 'claude',
  label: 'Claude Code',
  required: false,
  agent: true,
  features: ['claudeTabs'],
  floor: '',
  path: '',
  version: '',
  origin: { kind: 'none' },
  status: 'missing',
  offer: { action: 'install', package: 'Claude Code', route: 'vendor' }
}
const codex: DepEntry = {
  ...claude,
  name: 'codex',
  label: 'Codex',
  features: ['codexTabs'],
  offer: { action: 'install', package: 'Codex', route: 'vendor' }
}
const report = (...deps: DepEntry[]): DepsReport => ({
  os: { family: 'macos', id: 'macos', version: '15' },
  deps
})

test('agents: label, group, missing and the vendor route', () => {
  assert.equal(depLabel(claude), 'Claude Code')
  assert.equal(depLabel(git), 'git')
  assert.equal(featuresText(claude.features), 'Used by Claude Code tabs.')
  assert.deepEqual(agentDeps(report(git, claude, codex)), [claude, codex])
  assert.equal(
    agentGroupText([claude, codex]),
    'Lemongrass runs Claude Code or Codex in its tabs. Install at least one.'
  )
  assert.equal(anyAgentReady(report(claude, codex)), false)
  assert.equal(anyAgentReady(report(claude, { ...codex, status: 'ok', offer: null })), true)
  assert.equal(agentMissing(report(claude), 'claude'), true)
  assert.equal(agentMissing(report({ ...claude, status: 'ok' }), 'claude'), false)
  assert.equal(agentMissing(null, 'claude'), false)
  assert.equal(agentMissing(report(claude), 'agent'), false)
  assert.equal(routeBlocked(claude.offer!, 'Installing system packages is not supported.'), null)
  assert.match(routeNote(claude, 'macos') ?? '', /Anthropic's official installer/)
  assert.match(routeNote(codex, 'macos') ?? '', /shell profile/)
  assert.match(routeNote(codex, 'windows') ?? '', /PATH/)
  assert.equal(
    originText({ ...codex, origin: { kind: 'package', package: 'Codex', manager: 'vendor' } }),
    'from its official installer'
  )
  assert.equal(
    originText({
      ...codex,
      origin: { kind: 'package', package: '@openai/codex', manager: 'npm' }
    }),
    'from npm, @openai/codex'
  )
  assert.match(
    offPathText(claude, '/Users/a/.local/bin'),
    /add \/Users\/a\/.local\/bin to your shell profile/
  )
})

test('a Windows origin names where the tool came from', () => {
  const pkg = (manager: 'winget' | 'scoop' | 'installer' | 'windows', name: string): DepEntry => ({
    ...git,
    origin: { kind: 'package', package: name, manager }
  })
  assert.equal(
    originText(pkg('winget', 'Google.PlatformTools')),
    'from winget, Google.PlatformTools'
  )
  assert.equal(originText(pkg('scoop', 'adb')), 'from Scoop')
  assert.equal(originText(pkg('installer', 'Git.Git')), 'from the Git for Windows installer')
  assert.equal(originText(pkg('windows', 'OpenSSH.Client')), 'part of Windows')
  assert.equal(
    osText({ family: 'windows', id: 'windows', version: '11 (build 22631)' }),
    'Windows 11 (build 22631)'
  )
})
