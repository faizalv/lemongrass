// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// @ts-ignore node:assert has no types in the node tsconfig
import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
// @ts-ignore node --test imports the .ts source by its extension
import { AGY_RESUMED_ENV, agyLaunch, quoteCommandPath } from './agyLaunch.ts'

function home(): string {
  return mkdtempSync(join(tmpdir(), 'agy-launch-'))
}

function settingsOf(args: string[]): Record<string, unknown> {
  assert.equal(args[0], '--settings')
  assert.equal(args.length, 2)
  return JSON.parse(args[1]) as Record<string, unknown>
}

test('a fresh agy tab trusts its project and chains the lemongrass statusline', () => {
  const launch = agyLaunch([], '/work/app', '/h/.lemongrass/bin/lgrassd', home(), 'linux')
  assert.deepEqual(settingsOf(launch.args), {
    trustedWorkspaces: ['/work/app'],
    statusLine: { type: 'command', command: '/h/.lemongrass/bin/lgrassd statusline --vendor agy' }
  })
  assert.deepEqual(launch.env, {})
})

test('a resumed agy launch carries a fresh token each time', () => {
  const dir = home()
  const first = agyLaunch(['--conversation', 'c1'], '/p', '/bin/lgrassd', dir, 'linux')
  const second = agyLaunch(['--conversation', 'c1'], '/p', '/bin/lgrassd', dir, 'linux')
  assert.match(first.env[AGY_RESUMED_ENV], /^[0-9a-f-]{36}$/)
  assert.notEqual(first.env[AGY_RESUMED_ENV], second.env[AGY_RESUMED_ENV])
})

test('the statusline off switch leaves the user statusline to agy', () => {
  const dir = home()
  mkdirSync(join(dir, '.lemongrass'))
  writeFileSync(join(dir, '.lemongrass', 'statusline-off'), '')
  const launch = agyLaunch([], '/p', '/bin/lgrassd', dir, 'linux')
  assert.deepEqual(settingsOf(launch.args), { trustedWorkspaces: ['/p'] })
})

test('with no lgrassd and no project an agy tab gets no settings', () => {
  assert.deepEqual(agyLaunch([], undefined, null, home(), 'linux').args, [])
})

test('an lgrassd path is quoted only when the shell needs it', () => {
  assert.equal(quoteCommandPath('/opt/it is/lgrassd', 'linux'), `'/opt/it is/lgrassd'`)
  assert.equal(quoteCommandPath(`/o'k/lgrassd`, 'darwin'), `'/o'\\''k/lgrassd'`)
})

test('on Windows an lgrassd path is caret escaped for cmd, never quoted', () => {
  assert.equal(quoteCommandPath('C:\\Users\\a\\lgrassd.exe', 'win32'), 'C:/Users/a/lgrassd.exe')
  assert.equal(
    quoteCommandPath('C:\\Users\\a b\\lgrassd.exe', 'win32'),
    'C:/Users/a^ b/lgrassd.exe'
  )
  assert.equal(
    quoteCommandPath('C:\\Users\\Tom & Kim (Work)\\lgrassd.exe', 'win32'),
    'C:/Users/Tom^ ^&^ Kim^ ^(Work^)/lgrassd.exe'
  )
  assert.equal(
    quoteCommandPath('C:\\Users\\a^b,c;d=e\\lgrassd.exe', 'win32'),
    'C:/Users/a^^b^,c^;d^=e/lgrassd.exe'
  )
})
