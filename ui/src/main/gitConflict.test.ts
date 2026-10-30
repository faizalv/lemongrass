// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { WRITE_TIMEOUT_MS, readEnv, run } from './gitRun.ts'
import type { GitRunner } from './gitMove.ts'
import {
  applyResolution,
  conflictBlockRanges,
  conflictBlocksLeft,
  operationRefs,
  pickSide,
  readConflict,
  saveWorking
} from './gitConflict.ts'

// The machine's own git config, such as commit signing or a conflict style, must not reach these repos.
process.env.GIT_CONFIG_GLOBAL = '/dev/null'
process.env.GIT_CONFIG_NOSYSTEM = '1'
process.env.GIT_AUTHOR_NAME = process.env.GIT_COMMITTER_NAME = 'Test'
process.env.GIT_AUTHOR_EMAIL = process.env.GIT_COMMITTER_EMAIL = 'test@example.com'

const scratch = mkdtempSync(join(tmpdir(), 'lg-gitconflict-'))
after(() => rmSync(scratch, { recursive: true, force: true }))
let repoCount = 0

const LINES = ['l1', 'l2', 'l3', 'l4', 'l5', 'l6', 'l7', 'l8']

function git(cwd: string, ...args: string[]): string {
  return execFileSync('git', args, { cwd, encoding: 'utf8' }).trim()
}

// For commands expected to stop on a conflict, which exit non-zero.
function gitMayFail(cwd: string, ...args: string[]): void {
  spawnSync('git', args, { cwd, encoding: 'utf8', env: { ...process.env, GIT_EDITOR: 'true' } })
}

function write(root: string, name: string, lines: string[]): void {
  writeFileSync(join(root, name), lines.join('\n') + '\n')
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

// main and feat both change a.txt's third line from the base commit.
function diverged(): string {
  const root = join(scratch, `repo${++repoCount}`)
  execFileSync('git', ['init', '-q', '-b', 'main', root])
  write(root, 'a.txt', LINES)
  commit(root, 'base')
  git(root, 'switch', '-q', '-c', 'feat')
  write(root, 'a.txt', ['l1', 'l2', 'feat3', ...LINES.slice(3)])
  commit(root, 'feat')
  git(root, 'switch', '-q', 'main')
  write(root, 'a.txt', ['l1', 'l2', 'main3', ...LINES.slice(3, 7), 'main8'])
  commit(root, 'main')
  return root
}

function unmerged(root: string): string {
  return git(root, 'ls-files', '-u')
}

test('a merge conflict has the current branch left and the merged branch right', async () => {
  const root = diverged()
  gitMayFail(root, 'merge', 'feat')
  const result = await readConflict(runner(root), root, 'a.txt')
  assert.equal(result.status, 'ok')
  if (result.status !== 'ok') return
  assert.equal(result.operation, 'merge')
  assert.equal(result.leftRef, 'main')
  assert.equal(result.rightRef, 'feat')
  assert.equal(result.finalNewline, true)
  assert.deepEqual(result.chunks, [
    { kind: 'clean', lines: ['l1', 'l2'], disk: { start: 1, end: 2 } },
    {
      kind: 'conflict',
      left: ['main3'],
      base: ['l3'],
      right: ['feat3'],
      disk: {
        start: 3,
        end: 7,
        left: { start: 4, end: 4 },
        base: null,
        right: { start: 6, end: 6 }
      }
    },
    { kind: 'clean', lines: ['l4', 'l5', 'l6', 'l7', 'main8'], disk: { start: 8, end: 12 } }
  ])
  const disk = readFileSync(join(root, 'a.txt'), 'utf8').split('\n')
  assert.equal(disk[3], 'main3')
  assert.equal(disk[5], 'feat3')
  assert.equal(result.working, readFileSync(join(root, 'a.txt'), 'utf8'))
  assert.deepEqual(
    result.workingChunks?.map((chunk) => chunk.kind),
    ['clean', 'conflict', 'clean']
  )
})

test('a rebase conflict puts the replayed branch left and the upstream right', async () => {
  const root = diverged()
  git(root, 'switch', '-q', 'feat')
  gitMayFail(root, 'rebase', 'main')
  const result = await readConflict(runner(root), root, 'a.txt')
  assert.equal(result.status, 'ok')
  if (result.status !== 'ok') return
  assert.equal(result.operation, 'rebase')
  assert.equal(result.leftRef, 'feat')
  assert.equal(result.rightRef, 'main')
  const conflict = result.chunks?.find((chunk) => chunk.kind === 'conflict')
  assert.equal(conflict?.kind, 'conflict')
  if (conflict?.kind !== 'conflict') return
  assert.deepEqual([conflict.left, conflict.right], [['feat3'], ['main3']])
  // On disk git writes ours (the upstream) first, so the left side's lines come second.
  assert.deepEqual(conflict.disk?.left, { start: 6, end: 6 })
  assert.deepEqual(conflict.disk?.right, { start: 4, end: 4 })
  const working = result.workingChunks?.find((chunk) => chunk.kind === 'conflict')
  assert.equal(working?.kind === 'conflict' && working.left[0], 'feat3')
})

test('taking the left side in a rebase takes the replayed commit and stages it', async () => {
  const root = diverged()
  git(root, 'switch', '-q', 'feat')
  gitMayFail(root, 'rebase', 'main')
  assert.deepEqual(await pickSide(runner(root), 'a.txt', 'left'), { ok: true })
  assert.equal(unmerged(root), '')
  assert.equal(readFileSync(join(root, 'a.txt'), 'utf8').split('\n')[2], 'feat3')
})

test('applying a result writes the file and stages it', async () => {
  const root = diverged()
  gitMayFail(root, 'merge', 'feat')
  const content = ['l1', 'l2', 'both3', ...LINES.slice(3)].join('\n') + '\n'
  assert.deepEqual(await applyResolution(runner(root), root, 'a.txt', content), { ok: true })
  assert.equal(unmerged(root), '')
  assert.equal(readFileSync(join(root, 'a.txt'), 'utf8'), content)
  assert.equal(git(root, 'diff', '--cached', '--name-only'), 'a.txt')
})

test('a path that is not in conflict is neither read nor written', async () => {
  const root = diverged()
  assert.deepEqual(await readConflict(runner(root), root, 'a.txt'), { status: 'notConflicted' })
  const result = await applyResolution(runner(root), root, 'a.txt', 'x\n')
  assert.equal(result.ok, false)
  assert.equal(readFileSync(join(root, 'a.txt'), 'utf8').split('\n')[2], 'main3')
})

test('a hand-edited working file leaves the chunks without disk ranges', async () => {
  const root = diverged()
  gitMayFail(root, 'merge', 'feat')
  write(root, 'a.txt', ['l1', 'l2', 'edited', ...LINES.slice(3)])
  const result = await readConflict(runner(root), root, 'a.txt')
  assert.equal(result.status, 'ok')
  if (result.status !== 'ok') return
  assert.ok(result.chunks?.every((chunk) => chunk.disk === null))
  assert.equal(result.workingChunks, null)
})

test('deleted on one side and changed on the other is a pick-a-side case', async () => {
  const root = diverged()
  git(root, 'switch', '-q', 'feat')
  git(root, 'rm', '-q', 'a.txt')
  commit(root, 'drop a')
  git(root, 'switch', '-q', 'main')
  gitMayFail(root, 'merge', 'feat')
  const result = await readConflict(runner(root), root, 'a.txt')
  assert.equal(result.status, 'ok')
  if (result.status !== 'ok') return
  assert.equal(result.left.state, 'text')
  assert.deepEqual(result.right, { state: 'missing' })
  assert.equal(result.chunks, null)
  assert.deepEqual(await pickSide(runner(root), 'a.txt', 'right'), { ok: true })
  assert.equal(unmerged(root), '')
  assert.equal(existsSync(join(root, 'a.txt')), false)
})

test('added on both sides has no base and is a pick-a-side case', async () => {
  const root = diverged()
  git(root, 'switch', '-q', 'feat')
  write(root, 'new.txt', ['feat'])
  commit(root, 'feat new')
  git(root, 'switch', '-q', 'main')
  write(root, 'new.txt', ['main'])
  commit(root, 'main new')
  gitMayFail(root, 'merge', 'feat')
  const result = await readConflict(runner(root), root, 'new.txt')
  assert.equal(result.status, 'ok')
  if (result.status !== 'ok') return
  assert.deepEqual(result.base, { state: 'missing' })
  assert.equal(result.chunks, null)
})

test('a binary conflict is a pick-a-side case', async () => {
  const root = diverged()
  writeFileSync(join(root, 'b.bin'), Buffer.from([0, 1, 2]))
  commit(root, 'bin')
  git(root, 'switch', '-q', 'feat')
  git(root, 'merge', '-q', 'main', '--no-edit', '-X', 'theirs')
  writeFileSync(join(root, 'b.bin'), Buffer.from([0, 9, 9]))
  commit(root, 'feat bin')
  git(root, 'switch', '-q', 'main')
  writeFileSync(join(root, 'b.bin'), Buffer.from([0, 7, 7]))
  commit(root, 'main bin')
  gitMayFail(root, 'merge', 'feat')
  const result = await readConflict(runner(root), root, 'b.bin')
  assert.equal(result.status, 'ok')
  if (result.status !== 'ok') return
  assert.deepEqual([result.left, result.right], [{ state: 'binary' }, { state: 'binary' }])
  assert.equal(result.chunks, null)
})

test('a stash apply conflict with nothing stopped is the stash operation', async () => {
  const root = diverged()
  write(root, 'a.txt', ['l1', 'l2', 'mine3', ...LINES.slice(3)])
  git(root, 'stash', 'push', '-q')
  gitMayFail(root, 'switch', 'feat')
  gitMayFail(root, 'stash', 'apply')
  const result = await readConflict(runner(root), root, 'a.txt')
  assert.equal(result.status, 'ok')
  if (result.status !== 'ok') return
  assert.equal(result.operation, 'stash')
  assert.deepEqual([result.leftRef, result.rightRef], ['stashed changes', 'feat'])
  const conflict = result.chunks?.find((chunk) => chunk.kind === 'conflict')
  assert.ok(conflict?.kind === 'conflict' && conflict.left[0] === 'mine3')
})

test('a working file written with diff3 markers places the base lines too', async () => {
  const root = diverged()
  gitMayFail(root, '-c', 'merge.conflictStyle=diff3', 'merge', 'feat')
  const result = await readConflict(runner(root), root, 'a.txt')
  assert.equal(result.status, 'ok')
  if (result.status !== 'ok') return
  const conflict = result.chunks?.find((chunk) => chunk.kind === 'conflict')
  assert.ok(conflict?.kind === 'conflict')
  if (conflict?.kind !== 'conflict') return
  assert.deepEqual(conflict.disk, {
    start: 3,
    end: 9,
    left: { start: 4, end: 4 },
    base: { start: 6, end: 6 },
    right: { start: 8, end: 8 }
  })
})

test('a stopped merge and rebase are described by the same refs as the panes', async () => {
  const merging = diverged()
  gitMayFail(merging, 'merge', 'feat')
  assert.deepEqual(await operationRefs(runner(merging), 'merge'), { from: 'feat', onto: 'main' })
  const rebasing = diverged()
  git(rebasing, 'switch', '-q', 'feat')
  gitMayFail(rebasing, 'rebase', 'main')
  assert.deepEqual(await operationRefs(runner(rebasing), 'rebase'), { from: 'feat', onto: 'main' })
})

test('the conflict blocks left are counted from the working file markers', () => {
  const root = diverged()
  gitMayFail(root, 'merge', 'feat')
  assert.equal(conflictBlocksLeft(root, 'a.txt'), 1)
  write(root, 'a.txt', LINES)
  assert.equal(conflictBlocksLeft(root, 'a.txt'), 0)
  write(root, 'a.txt', ['<<<<<<< HEAD', 'open'])
  assert.equal(conflictBlocksLeft(root, 'a.txt'), null)
  assert.equal(conflictBlocksLeft(root, 'missing.txt'), null)
})

test('saving partial work writes the file and leaves it in conflict', async () => {
  const root = diverged()
  gitMayFail(root, 'merge', 'feat')
  const content = ['l1', 'l2', 'partial3', ...LINES.slice(3)].join('\n') + '\n'
  assert.deepEqual(await saveWorking(runner(root), root, 'a.txt', content), { ok: true })
  assert.equal(readFileSync(join(root, 'a.txt'), 'utf8'), content)
  assert.notEqual(unmerged(root), '')
})

test('saving refuses a path that is not in conflict', async () => {
  const root = diverged()
  const result = await saveWorking(runner(root), root, 'a.txt', 'x\n')
  assert.equal(result.ok, false)
  assert.equal(readFileSync(join(root, 'a.txt'), 'utf8').split('\n')[2], 'main3')
})

test('the marker blocks left are given with their line ranges', () => {
  const root = diverged()
  write(root, 'b.txt', [
    'top',
    '<<<<<<< HEAD',
    'a',
    '=======',
    'b',
    '>>>>>>> feat',
    'middle',
    '<<<<<<< HEAD',
    '=======',
    'c',
    '>>>>>>> feat'
  ])
  assert.deepEqual(conflictBlockRanges(root, 'b.txt'), [
    { start: 2, end: 6 },
    { start: 8, end: 11 }
  ])
  assert.deepEqual(conflictBlockRanges(root, 'a.txt'), [])
})
