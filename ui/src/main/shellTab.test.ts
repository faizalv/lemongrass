// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// @ts-ignore node:assert has no types in the node tsconfig
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import { chmodSync, mkdirSync, mkdtempSync, readdirSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { delimiter, join } from 'node:path'
import { test } from 'node:test'
// @ts-ignore node --test imports the .ts source by its extension
import {
  SHELL_TAB_ENV,
  shellInit,
  shellTabEnv,
  shellTabLaunch,
  writeShellShims
} from './shellTab.ts'

function onPath(name: string): string | undefined {
  const found = spawnSync('sh', ['-c', `command -v ${name}`], { encoding: 'utf8' })
  return found.status === 0 ? found.stdout.trim() : undefined
}

// A home whose rc files put a fake real `claude` and `codex` in front of PATH, the way an installer's line does.
function homeWithReorderingRc(): string {
  const home = mkdtempSync(join(tmpdir(), 'shellhome-'))
  const bin = join(home, 'bin')
  mkdirSync(bin)
  for (const name of ['claude', 'codex']) {
    writeFileSync(join(bin, name), '#!/bin/sh\necho REAL-AGENT\n')
    chmodSync(join(bin, name), 0o755)
  }
  const line = 'export PATH="$HOME/bin:$PATH"\nalias claude="$HOME/bin/claude"\n'
  writeFileSync(join(home, '.bashrc'), line)
  writeFileSync(join(home, '.zshrc'), line)
  mkdirSync(join(home, '.config', 'fish'), { recursive: true })
  writeFileSync(join(home, '.config', 'fish', 'config.fish'), 'set -gx PATH $HOME/bin $PATH\n')
  return home
}

function runShell(file: string, home: string, input: string): string {
  const launch = shellTabLaunch(file, [], { HOME: home, PATH: '/usr/bin:/bin' }, home, 'linux')
  const result = spawnSync(file, [...launch.args, '-i'], {
    env: { ...launch.env, XDG_CONFIG_HOME: join(home, '.config') } as Record<string, string>,
    input,
    encoding: 'utf8',
    timeout: 10_000
  })
  return `${result.stdout}${result.stderr}`
}

const TYPED = 'claude\ncodex\ncommand claude\nexit\n'

for (const shell of ['bash', 'zsh', 'fish']) {
  const file = onPath(shell)
  test(`${shell}: the agents stay blocked after an rc that reorders PATH`, { skip: !file }, () => {
    const out = runShell(file!, homeWithReorderingRc(), TYPED)
    assert.doesNotMatch(out, /REAL-AGENT/)
    assert.equal(out.match(/Shell tab/g)?.length, 3, out)
  })
}

test(
  'the shims shadow claude and codex and exit nonzero',
  { skip: process.platform === 'win32' },
  () => {
    const dir = writeShellShims(mkdtempSync(join(tmpdir(), 'shellhome-')), 'linux')
    assert.ok(dir)
    for (const name of ['claude', 'codex']) {
      assert.throws(
        () =>
          execFileSync(name, [], {
            env: { PATH: `${dir}${delimiter}/usr/bin:/bin` },
            stdio: 'pipe'
          }),
        (err: { status: number; stderr: Buffer }) =>
          err.status === 1 && err.stderr.toString().includes('New shell')
      )
    }
  }
)

test('Windows gets cmd shims and no posix init files', () => {
  const dir = writeShellShims(mkdtempSync(join(tmpdir(), 'shellhome-')), 'win32')
  assert.ok(dir)
  assert.deepEqual(
    readdirSync(dir)
      .filter((name) => name !== 'zsh')
      .sort(),
    ['claude.cmd', 'codex.cmd']
  )
})

test('each shell is started through its own init', () => {
  assert.deepEqual(shellInit('/bin/bash', '/d', {}).args, ['--rcfile', '/d/bashrc'])
  assert.equal(shellInit('/usr/bin/zsh', '/d', {}).env.ZDOTDIR, '/d/zsh')
  assert.equal(shellInit('/usr/bin/zsh', '/d', { ZDOTDIR: '/u' }).env.LGRASS_USER_ZDOTDIR, '/u')
  assert.equal(shellInit('/usr/bin/fish', '/d', {}).args[0], '--init-command')
  const pwsh = shellInit('C:\\Program Files\\PowerShell\\7\\pwsh.exe', 'C:\\d', {})
  assert.deepEqual(pwsh.args.slice(0, 2), ['-NoExit', '-Command'])
  assert.match(pwsh.args[2], /function global:claude/)
  assert.deepEqual(shellInit('C:\\Windows\\System32\\cmd.exe', 'C:\\d', {}), { args: [], env: {} })
})

test('the shim dir goes first on PATH, the marker is set and a tab id is dropped', () => {
  const env = shellTabEnv({ PATH: '/usr/bin', HOME: '/h', LGRASS_TAB_ID: 'abc' }, '/h/shellbin')
  assert.equal(env.PATH, `/h/shellbin${delimiter}/usr/bin`)
  assert.equal(env[SHELL_TAB_ENV], '1')
  assert.equal(env.HOME, '/h')
  assert.equal(env.LGRASS_TAB_ID, undefined)
})

test('a Windows Path key is prefixed in place', () => {
  const env = shellTabEnv({ Path: 'C:\\bin' }, 'C:\\shellbin')
  assert.equal(env.Path, `C:\\shellbin${delimiter}C:\\bin`)
  assert.equal(env.PATH, undefined)
})

test('a failed shim write still sets the marker', () => {
  const env = shellTabEnv({ PATH: '/usr/bin' }, undefined)
  assert.equal(env.PATH, '/usr/bin')
  assert.equal(env[SHELL_TAB_ENV], '1')
})
