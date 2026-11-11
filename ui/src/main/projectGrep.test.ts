// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { after, test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { readEnv } from './gitRun.ts'
import { grepProject as grepWith } from './projectGrepCore.ts'

const git = { binary: 'git', env: { ...readEnv, GIT_LITERAL_PATHSPECS: '0' } }
const grepProject = (root: string, query: string): ReturnType<typeof grepWith> =>
  grepWith(root, query, git)

const roots: string[] = []
after(() => roots.forEach((root) => rmSync(root, { recursive: true, force: true })))

function project(repo: boolean): string {
  const root = mkdtempSync(join(tmpdir(), 'lg-grep-'))
  roots.push(root)
  mkdirSync(join(root, 'src'))
  mkdirSync(join(root, 'biblio/books'), { recursive: true })
  mkdirSync(join(root, 'build'))
  writeFileSync(join(root, '.gitignore'), 'biblio/\nbuild/\n')
  writeFileSync(join(root, 'src/repo.go'), 'package x\n\tsite := r.GetById(id)\n')
  writeFileSync(join(root, 'untracked.go'), 'getbyid here\n')
  writeFileSync(join(root, 'biblio/books/note.md'), 'GetById is documented\n')
  writeFileSync(join(root, 'build/out.js'), 'GetById ignored\n')
  if (repo) {
    execFileSync('git', ['init', '-q'], { cwd: root })
    execFileSync('git', ['add', 'src', '.gitignore'], { cwd: root })
  }
  return root
}

for (const repo of [true, false]) {
  test(`finds code, untracked and biblio lines and skips ignored ones (${repo ? 'repo' : 'no git'})`, async () => {
    const root = project(repo)
    const result = await grepProject(root, 'GetById')
    const found = result.hits.map((hit) => `${hit.path}:${hit.line}`).sort()
    assert.deepEqual(found, ['biblio/books/note.md:1', 'src/repo.go:2', 'untracked.go:1'])
    const code = result.hits.find((hit) => hit.path === 'src/repo.go')
    assert.equal(code?.text, 'site := r.GetById(id)')
    assert.equal(result.truncated, false)
  })
}

test('a newer search cancels the older one', async () => {
  const root = project(true)
  const [older, newer] = await Promise.all([
    grepProject(root, 'GetById'),
    grepProject(root, 'package')
  ])
  assert.equal(older.cancelled, true)
  assert.deepEqual(
    newer.hits.map((hit) => hit.path),
    ['src/repo.go']
  )
})

test('a one-character query searches nothing', async () => {
  assert.deepEqual((await grepProject(project(true), 'G')).hits, [])
})
