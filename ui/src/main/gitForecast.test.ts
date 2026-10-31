// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { WRITE_TIMEOUT_MS, readEnv, run } from './gitRun.ts'
import type { GitRunner } from './gitMove.ts'
import { forecastMerge, forecastRebase } from './gitForecast.ts'

// The machine's own git config, such as commit signing or a conflict style, must not reach these repos.
process.env.GIT_CONFIG_GLOBAL = '/dev/null'
process.env.GIT_CONFIG_NOSYSTEM = '1'
process.env.GIT_AUTHOR_NAME = process.env.GIT_COMMITTER_NAME = 'Test'
process.env.GIT_AUTHOR_EMAIL = process.env.GIT_COMMITTER_EMAIL = 'test@example.com'

const scratch = mkdtempSync(join(tmpdir(), 'lg-gitforecast-'))
after(() => rmSync(scratch, { recursive: true, force: true }))
let repoCount = 0

function git(cwd: string, ...args: string[]): string {
  return execFileSync('git', args, { cwd, encoding: 'utf8' }).trim()
}

function runner(root: string): GitRunner {
  return {
    read: (args) => run(root, args, { env: readEnv }),
    write: (args) => run(root, args, { timeout: WRITE_TIMEOUT_MS, env: { GIT_EDITOR: 'true' } })
  }
}

function commitFile(root: string, name: string, text: string, message = text): void {
  writeFileSync(join(root, name), `${text}\n`)
  git(root, 'add', '-A')
  git(root, 'commit', '-q', '-m', message)
}

// main has a.txt and b.txt; feat branches from it and tracks main, so main is its upstream.
function repo(): string {
  const root = join(scratch, `repo${++repoCount}`)
  execFileSync('git', ['init', '-q', '-b', 'main', root])
  commitFile(root, 'a.txt', 'a base')
  commitFile(root, 'b.txt', 'b base')
  git(root, 'switch', '-q', '-c', 'feat')
  git(root, 'branch', '-q', '--set-upstream-to=main')
  return root
}

// Everything a forecast must leave alone.
function snapshot(root: string): string {
  return [
    git(root, 'rev-parse', 'HEAD'),
    git(root, 'for-each-ref'),
    git(root, 'status', '--porcelain=v2', '--branch')
  ].join('\n')
}

// What git itself does, to hold each forecast against.
function realStop(root: string, ...args: string[]): string[] {
  spawnSync('git', args, {
    cwd: root,
    encoding: 'utf8',
    env: { ...process.env, GIT_EDITOR: 'true' }
  })
  const unmerged = git(root, 'diff', '--name-only', '--diff-filter=U')
  return unmerged ? unmerged.split('\n') : []
}

test('a merge with no overlapping change forecasts clean and changes nothing', async () => {
  const root = repo()
  commitFile(root, 'f.txt', 'feat')
  git(root, 'switch', '-q', 'main')
  commitFile(root, 'm.txt', 'main')
  const before = snapshot(root)
  assert.deepEqual(await forecastMerge(runner(root), 'feat'), { status: 'clean' })
  assert.equal(snapshot(root), before)
})

test('a merge that conflicts lists the conflicting paths, as git merge then stops on', async () => {
  const root = repo()
  commitFile(root, 'a.txt', 'a feat')
  commitFile(root, 'f.txt', 'feat only')
  git(root, 'switch', '-q', 'main')
  commitFile(root, 'a.txt', 'a main')
  const before = snapshot(root)
  const forecast = await forecastMerge(runner(root), 'feat')
  assert.equal(snapshot(root), before)
  assert.deepEqual(forecast, { status: 'conflict', paths: ['a.txt'], stop: null })
  assert.deepEqual(realStop(root, 'merge', '--no-edit', 'feat'), ['a.txt'])
})

test('a merge where one side deleted a file the other changed lists that file', async () => {
  const root = repo()
  commitFile(root, 'b.txt', 'b feat')
  git(root, 'switch', '-q', 'main')
  git(root, 'rm', '-q', 'b.txt')
  git(root, 'commit', '-q', '-m', 'drop b')
  const forecast = await forecastMerge(runner(root), 'feat')
  assert.deepEqual(forecast, { status: 'conflict', paths: ['b.txt'], stop: null })
  assert.deepEqual(realStop(root, 'merge', '--no-edit', 'feat'), ['b.txt'])
})

test('a merge of binary files changed on both sides lists the binary file', async () => {
  const root = repo()
  writeFileSync(join(root, 'img.bin'), Buffer.from([0, 1, 2, 3]))
  git(root, 'add', '-A')
  git(root, 'commit', '-q', '-m', 'binary')
  git(root, 'switch', '-q', 'main')
  writeFileSync(join(root, 'img.bin'), Buffer.from([0, 9, 9, 9]))
  git(root, 'add', '-A')
  git(root, 'commit', '-q', '-m', 'other binary')
  const forecast = await forecastMerge(runner(root), 'feat')
  assert.deepEqual(forecast, { status: 'conflict', paths: ['img.bin'], stop: null })
})

test('a merge forecast git cannot work out is unavailable', async () => {
  const root = repo()
  const forecast = await forecastMerge(runner(root), 'no-such-branch')
  assert.equal(forecast.status, 'unavailable')
})

test('a rebase with no overlapping change forecasts clean and changes nothing', async () => {
  const root = repo()
  commitFile(root, 'f.txt', 'feat one')
  commitFile(root, 'g.txt', 'feat two')
  git(root, 'switch', '-q', 'main')
  commitFile(root, 'm.txt', 'main')
  git(root, 'switch', '-q', 'feat')
  const before = snapshot(root)
  assert.deepEqual(await forecastRebase(runner(root), 'feat'), { status: 'clean' })
  assert.equal(snapshot(root), before)
})

test('a rebase that conflicts on its first commit names that commit and its paths', async () => {
  const root = repo()
  commitFile(root, 'a.txt', 'a feat', 'feat edits a')
  commitFile(root, 'f.txt', 'feat only')
  git(root, 'switch', '-q', 'main')
  commitFile(root, 'a.txt', 'a main')
  git(root, 'switch', '-q', 'feat')
  const before = snapshot(root)
  const forecast = await forecastRebase(runner(root), 'feat')
  assert.equal(snapshot(root), before)
  assert.deepEqual(forecast, {
    status: 'conflict',
    paths: ['a.txt'],
    stop: { index: 1, total: 2, subject: 'feat edits a' }
  })
  assert.deepEqual(realStop(root, 'rebase', 'main'), ['a.txt'])
})

test('a rebase that conflicts on a later commit carries the earlier commits forward', async () => {
  const root = repo()
  commitFile(root, 'f.txt', 'feat one')
  commitFile(root, 'b.txt', 'b feat', 'feat edits b')
  commitFile(root, 'g.txt', 'feat three')
  git(root, 'switch', '-q', 'main')
  commitFile(root, 'b.txt', 'b main')
  git(root, 'switch', '-q', 'feat')
  const forecast = await forecastRebase(runner(root), 'feat')
  assert.deepEqual(forecast, {
    status: 'conflict',
    paths: ['b.txt'],
    stop: { index: 2, total: 3, subject: 'feat edits b' }
  })
  assert.deepEqual(realStop(root, 'rebase', 'main'), ['b.txt'])
})

test('a rebase does not count a commit whose change the upstream already has', async () => {
  const root = repo()
  commitFile(root, 'a.txt', 'a shared', 'shared change')
  commitFile(root, 'b.txt', 'b feat', 'feat edits b')
  const shared = git(root, 'rev-parse', 'HEAD~1')
  git(root, 'switch', '-q', 'main')
  git(root, 'cherry-pick', shared)
  commitFile(root, 'b.txt', 'b main')
  git(root, 'switch', '-q', 'feat')
  const forecast = await forecastRebase(runner(root), 'feat')
  assert.deepEqual(forecast, {
    status: 'conflict',
    paths: ['b.txt'],
    stop: { index: 1, total: 1, subject: 'feat edits b' }
  })
  assert.deepEqual(realStop(root, 'rebase', 'main'), ['b.txt'])
})

test('a rebase does not replay merge commits', async () => {
  const root = repo()
  git(root, 'switch', '-q', '-c', 'side')
  commitFile(root, 's.txt', 'side')
  git(root, 'switch', '-q', 'feat')
  commitFile(root, 'f.txt', 'feat')
  git(root, 'merge', '-q', '--no-edit', '--no-ff', 'side')
  commitFile(root, 'a.txt', 'a feat', 'feat edits a')
  git(root, 'switch', '-q', 'main')
  commitFile(root, 'a.txt', 'a main')
  git(root, 'switch', '-q', 'feat')
  const forecast = await forecastRebase(runner(root), 'feat')
  assert.deepEqual(forecast, {
    status: 'conflict',
    paths: ['a.txt'],
    stop: { index: 3, total: 3, subject: 'feat edits a' }
  })
  assert.deepEqual(realStop(root, 'rebase', 'main'), ['a.txt'])
})

test('a rebase forecast for a branch that is not checked out works the same', async () => {
  const root = repo()
  commitFile(root, 'a.txt', 'a feat', 'feat edits a')
  git(root, 'switch', '-q', 'main')
  commitFile(root, 'a.txt', 'a main')
  const before = snapshot(root)
  const forecast = await forecastRebase(runner(root), 'feat')
  assert.equal(snapshot(root), before)
  assert.equal(forecast.status, 'conflict')
})

test('a branch with no upstream has no rebase forecast', async () => {
  const root = repo()
  git(root, 'branch', '-q', 'solo')
  const forecast = await forecastRebase(runner(root), 'solo')
  assert.deepEqual(forecast, {
    status: 'unavailable',
    error: 'solo has no upstream to rebase onto.'
  })
})
