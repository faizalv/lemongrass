// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { BIBLIO_EXPAND_LIMIT, searchBiblio, type BiblioSearchNode } from './biblioSearch.ts'

function dir(path: string, children: BiblioSearchNode[]): BiblioSearchNode {
  return { name: path.split('/').pop() ?? path, path, type: 'dir', children }
}

function file(path: string): BiblioSearchNode {
  return { name: path.split('/').pop() ?? path, path, type: 'file' }
}

const tree: BiblioSearchNode[] = [
  dir('books', [dir('books/os_dependencies', [file('books/os_dependencies/windows.md')])]),
  dir('laws', [file('laws/no_em_dash.md')]),
  dir('scratchpad', [
    dir('scratchpad/archive', [
      dir('scratchpad/archive/mac-window-controls', [
        file('scratchpad/archive/mac-window-controls/prd.md')
      ]),
      dir('scratchpad/archive/lgrass-windows-gate', [
        file('scratchpad/archive/lgrass-windows-gate/prd.md')
      ])
    ]),
    dir('scratchpad/osdeps-windows', [file('scratchpad/osdeps-windows/prd.md')])
  ])
]

function shown(list: BiblioSearchNode[]): string[] {
  const out: string[] = []
  for (const node of list) {
    out.push(node.path)
    if (node.children) out.push(...shown(node.children))
  }
  return out
}

test('a query is a case-insensitive substring of the shown name, never fuzzy', () => {
  const result = searchBiblio(tree, 'WINDOWS', 'all', true)
  assert.deepEqual([...result.matches.keys()].sort(), [
    'books/os_dependencies/windows.md',
    'scratchpad/archive/lgrass-windows-gate',
    'scratchpad/osdeps-windows'
  ])
  assert.deepEqual(result.matches.get('scratchpad/osdeps-windows'), [[7, 14]])
})

test('the tree is cut down to the matches, every category row kept in place', () => {
  const result = searchBiblio(tree, 'windows', 'all', false)
  assert.deepEqual(shown(result.tree), [
    'books',
    'books/os_dependencies',
    'books/os_dependencies/windows.md',
    'laws',
    'scratchpad',
    'scratchpad/osdeps-windows',
    'scratchpad/osdeps-windows/prd.md'
  ])
  assert.deepEqual([...result.noMatch], ['laws'])
})

test('the archive is skipped unless asked for, and then comes after the live tasks', () => {
  const without = searchBiblio(tree, 'windows', 'all', false)
  assert.ok(!without.matches.has('scratchpad/archive/lgrass-windows-gate'))
  assert.ok(!without.expand['scratchpad/archive'])
  assert.equal(searchBiblio(tree, 'archive', 'all', true).matches.size, 0)
  const scratchpad = searchBiblio(tree, 'windows', 'all', true).tree[2]
  assert.deepEqual(
    scratchpad.children?.map((node) => node.path),
    ['scratchpad/osdeps-windows', 'scratchpad/archive']
  )
})

test('siblings sort closest match first, a shorter name winning a tie, then tree order', () => {
  const nodes = [
    dir('laws', [
      file('laws/no_windows_paths.md'),
      file('laws/sidewindows.md'),
      file('laws/windows_first.md'),
      file('laws/windows.md'),
      file('laws/windows_build.md')
    ])
  ]
  assert.deepEqual(searchBiblio(nodes, 'windows', 'all', true).order, [
    'laws/windows.md',
    'laws/windows_first.md',
    'laws/windows_build.md',
    'laws/no_windows_paths.md',
    'laws/sidewindows.md'
  ])
})

test('a folder ranks by its best match below it', () => {
  const nodes = [
    dir('scratchpad', [
      dir('scratchpad/a-task', [file('scratchpad/a-task/xwindowsx.md')]),
      dir('scratchpad/b-task', [file('scratchpad/b-task/windows.md')])
    ])
  ]
  assert.deepEqual(
    searchBiblio(nodes, 'windows', 'all', true).tree[0].children?.map((node) => node.path),
    ['scratchpad/b-task', 'scratchpad/a-task']
  )
})

test('a matching folder keeps its whole content, matches first', () => {
  const nodes = [
    dir('scratchpad', [
      dir('scratchpad/windows-task', [
        file('scratchpad/windows-task/prd.md'),
        file('scratchpad/windows-task/windows.md')
      ])
    ])
  ]
  const result = searchBiblio(nodes, 'windows', 'all', true)
  assert.deepEqual(
    result.tree[0].children?.[0].children?.map((node) => node.path),
    ['scratchpad/windows-task/windows.md', 'scratchpad/windows-task/prd.md']
  )
})

test('only the folders above a match open, not the matching folder itself', () => {
  const result = searchBiblio(tree, 'windows', 'all', false)
  assert.deepEqual(Object.keys(result.expand).sort(), [
    'books',
    'books/os_dependencies',
    'scratchpad'
  ])
})

test('a scope limits where matches are found, and other categories show no members', () => {
  const result = searchBiblio(tree, 'windows', 'books', true)
  assert.deepEqual(result.order, ['books/os_dependencies/windows.md'])
  assert.deepEqual(result.tree[2].children, [])
  assert.equal(result.noMatch.size, 0)
})

test('the .md suffix is never matched', () => {
  assert.equal(searchBiblio(tree, '.md', 'all', true).matches.size, 0)
})

test('every occurrence in a name is highlighted', () => {
  const nodes = [dir('laws', [file('laws/aXaXa.md')])]
  assert.deepEqual(searchBiblio(nodes, 'a', 'all', true).matches.get('laws/aXaXa.md'), [
    [0, 1],
    [2, 3],
    [4, 5]
  ])
})

test('past the expand limit nothing opens', () => {
  const many = Array.from({ length: BIBLIO_EXPAND_LIMIT + 1 }, (_, i) => file(`laws/rule${i}.md`))
  const result = searchBiblio([dir('laws', many)], 'rule', 'all', true)
  assert.ok(result.capped)
  assert.deepEqual(result.expand, {})
  assert.equal(result.order.length, BIBLIO_EXPAND_LIMIT + 1)
})

test('a blank query matches nothing', () => {
  assert.equal(searchBiblio(tree, '   ', 'all', true).matches.size, 0)
})
