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
import { INTENT_MAX_COMMITS, readIntent } from './conflictIntent.ts'

// The machine's own git config, such as commit signing or a conflict style, must not reach these repos.
process.env.GIT_CONFIG_GLOBAL = '/dev/null'
process.env.GIT_CONFIG_NOSYSTEM = '1'
process.env.GIT_AUTHOR_NAME = process.env.GIT_COMMITTER_NAME = 'Test'
process.env.GIT_AUTHOR_EMAIL = process.env.GIT_COMMITTER_EMAIL = 'test@example.com'

const scratch = mkdtempSync(join(tmpdir(), 'lg-conflictintent-'))
after(() => rmSync(scratch, { recursive: true, force: true }))
let repoCount = 0

function git(cwd: string, ...args: string[]): string {
  return execFileSync('git', args, { cwd, encoding: 'utf8' }).trim()
}

function gitMayFail(cwd: string, ...args: string[]): void {
  spawnSync('git', args, { cwd, encoding: 'utf8', env: { ...process.env, GIT_EDITOR: 'true' } })
}

function write(root: string, name: string, text: string): void {
  writeFileSync(join(root, name), text)
}

function commit(root: string, message: string): void {
  git(root, 'add', '-A')
  git(root, 'commit', '-q', '-m', message)
}

function runner(root: string): GitRunner {
  return {
    read: (args) => run(root, args, { env: readEnv }),
    write: (args) =>
      run(root, args, { timeout: WRITE_TIMEOUT_MS, env: { GIT_LITERAL_PATHSPECS: '1' } })
  }
}

// main renames the constant, feat raises it, and each side also commits a file the conflict does not touch.
function diverged(): string {
  const root = join(scratch, `repo${++repoCount}`)
  execFileSync('git', ['init', '-q', '-b', 'main', root])
  write(root, 'tax.js', 'const TAX_RATE = 0.1\n')
  write(root, 'other.txt', 'base\n')
  commit(root, 'base')
  git(root, 'switch', '-q', '-c', 'feat')
  write(root, 'tax.js', 'const TAX_RATE = 0.2\n')
  commit(root, 'Raise the tax rate\n\nThe new law starts in May.')
  write(root, 'other.txt', 'feat\n')
  commit(root, 'Unrelated feat change')
  git(root, 'switch', '-q', 'main')
  write(root, 'tax.js', 'const TAX = 0.1\n')
  commit(root, 'Rename TAX_RATE to TAX')
  write(root, 'other.txt', 'main\n')
  commit(root, 'Unrelated main change')
  return root
}

function subjects(side: { commits: { subject: string }[] }): string[] {
  return side.commits.map((commit) => commit.subject)
}

test('a merge reads each side since the merge base, only commits touching the paths', async () => {
  const root = diverged()
  gitMayFail(root, 'merge', 'feat')
  const intent = await readIntent(runner(root), 'merge', ['tax.js'])
  assert.deepEqual(subjects(intent.left), ['Rename TAX_RATE to TAX'])
  assert.deepEqual(subjects(intent.right), ['Raise the tax rate'])
  assert.equal(intent.right.commits[0].body, 'The new law starts in May.')
  assert.match(intent.right.commits[0].hash, /^[0-9a-f]{7,}$/)
  assert.equal(intent.left.missing, null)
  assert.equal(intent.right.more, 0)
})

test('a rebase reads the replayed commit left and the upstream since the branch point right', async () => {
  const root = diverged()
  git(root, 'switch', '-q', 'feat')
  gitMayFail(root, 'rebase', 'main')
  const intent = await readIntent(runner(root), 'rebase', ['tax.js'])
  assert.deepEqual(subjects(intent.left), ['Raise the tax rate'])
  assert.deepEqual(subjects(intent.right), ['Rename TAX_RATE to TAX'])
})

test('a stash apply reads the stash message left and the branch since the stash right', async () => {
  const root = join(scratch, `repo${++repoCount}`)
  execFileSync('git', ['init', '-q', '-b', 'main', root])
  write(root, 'tax.js', 'const TAX_RATE = 0.1\n')
  commit(root, 'base')
  write(root, 'tax.js', 'const TAX_RATE = 0.3\n')
  git(root, 'stash', 'push', '-q', '-m', 'try a higher rate')
  write(root, 'tax.js', 'const TAX = 0.1\n')
  commit(root, 'Rename TAX_RATE to TAX')
  gitMayFail(root, 'stash', 'apply')
  const intent = await readIntent(runner(root), 'stash', ['tax.js'])
  assert.deepEqual(subjects(intent.left), ['On main: try a higher rate'])
  assert.deepEqual(subjects(intent.right), ['Rename TAX_RATE to TAX'])
})

test('a side with more commits than the cap says how many were left out', async () => {
  const root = diverged()
  for (let i = 0; i < INTENT_MAX_COMMITS + 2; i++) {
    write(root, 'tax.js', `const TAX = 0.${i + 2}\n`)
    commit(root, `Tune ${i}`)
  }
  gitMayFail(root, 'merge', 'feat')
  const intent = await readIntent(runner(root), 'merge', ['tax.js'])
  assert.equal(intent.left.commits.length, INTENT_MAX_COMMITS)
  assert.equal(intent.left.more, 3)
  assert.equal(intent.left.commits[0].subject, `Tune ${INTENT_MAX_COMMITS + 1}`)
})

test('without a stash git cannot tell what the stashed side meant', async () => {
  const root = diverged()
  const intent = await readIntent(runner(root), 'stash', ['tax.js'])
  assert.equal(intent.left.missing, 'no stash is left to read')
  assert.deepEqual(intent.left.commits, [])
})
