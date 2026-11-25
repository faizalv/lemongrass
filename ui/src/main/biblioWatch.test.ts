// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { appendFileSync, mkdirSync, mkdtempSync, renameSync, rmSync, writeFileSync } from 'fs'
import { tmpdir } from 'os'
import { join } from 'path'
import { watchTree } from './biblioWatch.ts'

const settle = (ms = 150): Promise<void> => new Promise((done) => setTimeout(done, ms))

// Counts change calls and reports whether a step produced at least one.
async function withWatch(
  run: (root: string, changedBy: (step: () => void) => Promise<boolean>) => Promise<void>
): Promise<void> {
  const root = mkdtempSync(join(tmpdir(), 'biblio-watch-'))
  mkdirSync(join(root, 'scratchpad', 'task'), { recursive: true })
  let changes = 0
  const watch = watchTree(
    root,
    () => changes++,
    () => {}
  )
  await settle()
  try {
    await run(root, async (step) => {
      changes = 0
      step()
      await settle()
      return changes > 0
    })
  } finally {
    watch.close()
    rmSync(root, { recursive: true })
  }
}

test('reports in-place edits to a file after it was replaced by rename', () =>
  withWatch(async (root, changedBy) => {
    const file = join(root, 'scratchpad', 'task', 'prd.md')
    assert.ok(await changedBy(() => writeFileSync(file, '- [ ] a\n')))
    assert.ok(
      await changedBy(() => {
        writeFileSync(`${file}.tmp`, '- [ ] a\n- [ ] b\n')
        renameSync(`${file}.tmp`, file)
      })
    )
    assert.ok(await changedBy(() => writeFileSync(file, '- [x] a\n- [ ] b\n')))
    assert.ok(await changedBy(() => appendFileSync(file, '- [ ] c\n')))
  }))

test('follows directories created after the watch started', () =>
  withWatch(async (root, changedBy) => {
    const dir = join(root, 'scratchpad', 'new-task', 'nested')
    assert.ok(await changedBy(() => mkdirSync(dir, { recursive: true })))
    assert.ok(await changedBy(() => writeFileSync(join(dir, 'notes.md'), 'x\n')))
    assert.ok(await changedBy(() => appendFileSync(join(dir, 'notes.md'), 'y\n')))
  }))

test('follows a directory moved within the tree', () =>
  withWatch(async (root, changedBy) => {
    const from = join(root, 'scratchpad', 'task')
    const to = join(root, 'scratchpad', 'archive-task')
    assert.ok(await changedBy(() => renameSync(from, to)))
    assert.ok(await changedBy(() => writeFileSync(join(to, 'activities.md'), 'z\n')))
  }))
