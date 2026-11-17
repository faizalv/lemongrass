// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, rmSync, utimesSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { byLatestActivity } from './biblioActivity.ts'

const root = mkdtempSync(join(tmpdir(), 'biblio-activity-'))
after(() => rmSync(root, { recursive: true, force: true }))

function touch(path: string, seconds: number): void {
  utimesSync(path, seconds, seconds)
}

test('a task edited anywhere below it sorts above newer tasks, archive pinned first', () => {
  mkdirSync(join(root, 'archive'))
  mkdirSync(join(root, 'old', 'images'), { recursive: true })
  mkdirSync(join(root, 'new'))
  writeFileSync(join(root, 'old', 'prd.md'), 'x')
  writeFileSync(join(root, 'old', 'images', 'a.png'), 'x')
  writeFileSync(join(root, 'new', 'prd.md'), 'x')
  for (const path of ['old/images/a.png', 'old/prd.md', 'old/images', 'old'])
    touch(join(root, path), 1000)
  for (const path of ['new/prd.md', 'new']) touch(join(root, path), 2000)
  touch(join(root, 'archive'), 500)

  const names = ['new', 'archive', 'old']
  assert.deepEqual(byLatestActivity(root, names, 'archive'), ['archive', 'new', 'old'])

  touch(join(root, 'old', 'prd.md'), 3000)
  assert.deepEqual(byLatestActivity(root, names, 'archive'), ['archive', 'old', 'new'])

  touch(join(root, 'new', 'prd.md'), 4000)
  touch(join(root, 'old', 'images', 'a.png'), 5000)
  assert.deepEqual(byLatestActivity(root, names, 'archive'), ['archive', 'old', 'new'])
})
