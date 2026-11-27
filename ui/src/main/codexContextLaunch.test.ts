// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { test, type TestContext } from 'node:test'
import { resumeArgs, spawnArgs } from '../renderer/src/shellAgents.ts'
import { codexContextArgs } from './codexContextLaunch.ts'

const LONG_CONTEXT = ['-c', 'model_context_window=1000000']

function fixture(t: TestContext): {
  write(relativePath: string, content: string): void
  contextArgs(args?: string[]): string[]
} {
  const root = mkdtempSync(join(tmpdir(), 'codex-context-launch-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const codexHome = join(root, 'codex-home')
  const cwd = join(root, 'project', 'src')
  const systemConfig = join(root, 'system-config.toml')
  mkdirSync(cwd, { recursive: true })
  function write(relativePath: string, content: string): void {
    const file = join(root, relativePath)
    mkdirSync(dirname(file), { recursive: true })
    writeFileSync(file, content)
  }
  function contextArgs(args: string[] = []): string[] {
    return codexContextArgs(args, cwd, codexHome, systemConfig)
  }
  return { write, contextArgs }
}

test('requests a million tokens for fresh, resumed, and thinker launches', (t) => {
  const { contextArgs } = fixture(t)
  const launches = [
    [],
    resumeArgs('codex', 'session-id') ?? [],
    ['-m', 'gpt-6.2-sol', 'Review the assigned module.']
  ]
  for (const launch of launches) {
    assert.deepEqual(contextArgs(spawnArgs('codex', 'tab-id', launch)), LONG_CONTEXT)
  }
})

test('preserves explicit context arguments on either side of resume', (t) => {
  const { contextArgs } = fixture(t)
  for (const args of [
    ['-c', 'model_context_window=500000', 'resume', 'session-id'],
    ['resume', 'session-id', '--config=model_context_window=500000'],
    ['-cmodel_context_window=500000']
  ]) {
    assert.deepEqual(contextArgs(args), [])
  }
  assert.deepEqual(contextArgs(['--', '-c', 'model_context_window=500000']), LONG_CONTEXT)
})

test('preserves explicit context in system, user, and project config files', async (t) => {
  for (const path of [
    'system-config.toml',
    'codex-home/config.toml',
    'project/.codex/config.toml',
    'project/src/.codex/config.toml',
    'project/src/config.toml'
  ]) {
    await t.test(path, (child) => {
      const { write, contextArgs } = fixture(child)
      write(path, '"model_context_window" = 500_000 # workstation preference\n')
      assert.deepEqual(contextArgs(), [])
    })
  }
})

test('honors the selected profile and ignores comments and other model settings', (t) => {
  const { write, contextArgs } = fixture(t)
  write('codex-home/config.toml', '# model_context_window = 500000\nmodel = "gpt-6.2-astra"\n')
  write('codex-home/work.config.toml', 'model_context_window = 500_000\n')
  assert.deepEqual(contextArgs(), LONG_CONTEXT)
  assert.deepEqual(contextArgs(['--profile=work']), [])
  write('codex-home/config.toml', 'profile = "work"\n')
  assert.deepEqual(contextArgs(), [])
})

test('preserves the target project setting when Codex receives a different directory', (t) => {
  const { write, contextArgs } = fixture(t)
  write('other-project/.codex/config.toml', 'model_context_window = 400000\n')
  assert.deepEqual(contextArgs(['-C', '../../other-project']), [])
  assert.deepEqual(contextArgs(), LONG_CONTEXT)
})

test('leaves custom model catalogs and providers in charge of their context', (t) => {
  const { write, contextArgs } = fixture(t)
  assert.deepEqual(contextArgs(['--oss']), [])
  assert.deepEqual(contextArgs(['-c', 'model_provider="custom"']), [])
  assert.deepEqual(contextArgs(['-c', 'model_provider="openai"']), LONG_CONTEXT)
  write('codex-home/config.toml', 'model_catalog_json = "my-models.json"\n')
  assert.deepEqual(contextArgs(), [])
})
