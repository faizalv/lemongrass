// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { resumeArgs, spawnArgs } from './shellAgents.ts'

test('a Codex tab bypasses its sandbox and approvals after the tab marker', () => {
  assert.deepEqual(spawnArgs('codex', 'tab-1'), [
    '--no-daemon',
    '-c',
    'shell_environment_policy.set.LGRASS_TAB_ID="tab-1"',
    '--dangerously-bypass-approvals-and-sandbox'
  ])
})

test('a resumed Codex tab keeps the bypass ahead of the resume subcommand', () => {
  const args = spawnArgs('codex', 'tab-1', resumeArgs('codex', 'session-1') ?? [])
  assert.deepEqual(args?.slice(-3), [
    '--dangerously-bypass-approvals-and-sandbox',
    'resume',
    'session-1'
  ])
})

test('a Claude tab skips its permission prompts', () => {
  assert.deepEqual(spawnArgs('claude', 'tab-1', ['--resume', 's']), [
    '--dangerously-skip-permissions',
    '--resume',
    's'
  ])
})

test('a plain shell gets no arguments', () => {
  assert.equal(spawnArgs('zsh', 'tab-1'), undefined)
})

test('an Antigravity tab skips its approvals ahead of the resume flag', () => {
  assert.deepEqual(spawnArgs('agy', 'tab-1', resumeArgs('agy', 'conv-1') ?? []), [
    '--dangerously-skip-permissions',
    '--conversation',
    'conv-1'
  ])
})

test('an Antigravity initial prompt stays last, after every flag', () => {
  const args = spawnArgs('agy', 'tab-1', ['--model', 'm', '-i', 'Read a.md'])
  assert.deepEqual(args, ['--dangerously-skip-permissions', '--model', 'm', '-i', 'Read a.md'])
})
