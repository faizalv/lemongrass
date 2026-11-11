// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  clipLine,
  GREP_LINE_CHARS,
  grepArgs,
  matchRanges,
  parseGrepRecord
} from './projectGrepCore.ts'

test('the repo pass searches untracked files and leaves biblio to its own pass', () => {
  const args = grepArgs('GetById', 'repo')
  assert.equal(args[0], 'grep')
  assert.ok(args.includes('--untracked'))
  assert.deepEqual(args.slice(-5), ['-e', 'GetById', '--', '.', ':(exclude)biblio'])
})

test('the biblio pass reads gitignored files', () => {
  const args = grepArgs('x', 'biblio')
  assert.ok(args.includes('--no-index'))
  assert.ok(!args.includes('--exclude-standard'))
  assert.equal(args.at(-1), 'biblio')
})

test('a query starting with a dash stays a pattern', () => {
  const args = grepArgs('-v', 'plain')
  assert.equal(args[args.indexOf('-e') + 1], '-v')
})

test('match ranges are case-insensitive and cover every occurrence', () => {
  assert.deepEqual(matchRanges('getById GETBYID', 'getbyid'), [
    [0, 7],
    [8, 15]
  ])
})

test('a line is trimmed and its matches shift with it', () => {
  assert.deepEqual(clipLine('\t  site := repo.GetById(id)  ', 'getbyid'), {
    text: 'site := repo.GetById(id)',
    ranges: [[13, 20]]
  })
})

test('a far-in match is kept in view behind an ellipsis', () => {
  const raw = 'a'.repeat(100) + 'NEEDLE' + 'b'.repeat(10)
  const { text, ranges } = clipLine(raw, 'needle')
  assert.ok(text.startsWith('…'))
  assert.equal(text.slice(ranges[0][0], ranges[0][1]), 'NEEDLE')
})

test('a long line is capped and drops matches past the cut', () => {
  const raw = 'x' + 'y'.repeat(GREP_LINE_CHARS + 50) + 'x'
  const { text, ranges } = clipLine(raw, 'x')
  assert.ok(text.endsWith('…'))
  assert.deepEqual(ranges, [[0, 1]])
})

test('a record parses into a project-relative hit', () => {
  assert.deepEqual(parseGrepRecord('ui/a.ts\x0012\x00  foo(bar)', 'foo'), {
    path: 'ui/a.ts',
    line: 12,
    text: 'foo(bar)',
    ranges: [[0, 3]]
  })
})

test('a malformed record is dropped', () => {
  assert.equal(parseGrepRecord('no separators', 'x'), null)
  assert.equal(parseGrepRecord('a.ts\x00zero\x00text', 'x'), null)
})
