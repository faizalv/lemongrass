// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ALL_FILE_LIMIT,
  biblioFilePaths,
  createDoubleShift,
  rankFiles,
  rowsFor,
  shortPath
} from './searchEverywhere.ts'
import { fuzzyMatch } from './treeSearch.ts'
import type { BiblioTree } from '../../preload/types'

function shifter(): {
  count: () => number
  at: (ms: number) => void
  s: ReturnType<typeof createDoubleShift>
} {
  let fired = 0
  let clock = 0
  const s = createDoubleShift(
    () => (fired += 1),
    () => clock
  )
  return { count: () => fired, at: (ms) => (clock = ms), s }
}

function tap(h: ReturnType<typeof shifter>, ms: number): void {
  h.at(ms)
  h.s.keydown({ key: 'Shift' })
  h.s.keyup({ key: 'Shift' })
}

test('two quick Shift taps trigger once', () => {
  const h = shifter()
  tap(h, 0)
  tap(h, 200)
  assert.equal(h.count(), 1)
  tap(h, 300)
  assert.equal(h.count(), 1)
})

test('slow taps do not trigger', () => {
  const h = shifter()
  tap(h, 0)
  tap(h, 600)
  assert.equal(h.count(), 0)
})

test('Shift used for a capital letter does not count', () => {
  const h = shifter()
  tap(h, 0)
  h.at(100)
  h.s.keydown({ key: 'Shift' })
  h.s.keydown({ key: 'A' })
  h.s.keyup({ key: 'Shift' })
  assert.equal(h.count(), 0)
})

test('a key between the taps breaks the pair', () => {
  const h = shifter()
  tap(h, 0)
  h.s.keydown({ key: 'x' })
  tap(h, 100)
  assert.equal(h.count(), 0)
})

test('biblio files become project paths, the toc included', () => {
  const tree: BiblioTree = {
    toc: { name: 'toc.md', path: 'books/toc.md', type: 'file' },
    children: [
      {
        name: 'books',
        path: 'books',
        type: 'dir',
        children: [{ name: 'a.md', path: 'books/a.md', type: 'file' }]
      }
    ]
  }
  assert.deepEqual(biblioFilePaths(tree), ['biblio/books/toc.md', 'biblio/books/a.md'])
  assert.deepEqual(biblioFilePaths(null), [])
})

test('a name match outranks a path spread over folders, with name highlights', () => {
  const rows = rankFiles(['src/m/s/t/x.ts', 'src/mst.ts', 'docs/readme.md'], 'mst', 10, fuzzyMatch)
  assert.equal(rows[0].path, 'src/mst.ts')
  assert.equal(rows[0].name, 'mst.ts')
  assert.equal(rows[0].dir, 'src')
  assert.deepEqual(rows[0].nameRanges, [[0, 3]])
  assert.equal(rows.length, 2)
})

test('All caps file names above the text lines', () => {
  const files = rankFiles(
    Array.from({ length: 20 }, (_, i) => `f${i}.ts`),
    'f',
    50,
    fuzzyMatch
  )
  const text = [{ path: 'a.ts', line: 1, text: 'f', ranges: [[0, 1]] as [number, number][] }]
  const all = rowsFor('all', files, text)
  assert.equal(all.length, ALL_FILE_LIMIT + 1)
  assert.equal(all.at(-1)?.kind, 'text')
  assert.equal(rowsFor('files', files, text).length, 20)
  assert.deepEqual(
    rowsFor('text', files, text).map((r) => r.kind),
    ['text']
  )
})

test('a short path keeps the last two segments', () => {
  assert.equal(shortPath('internal/usecase/rkh.go'), 'usecase/rkh.go')
  assert.equal(shortPath('a.go'), 'a.go')
})
