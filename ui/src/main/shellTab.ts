// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { mkdirSync, writeFileSync } from 'node:fs'
import { basename, delimiter, join } from 'node:path'
import type { Env } from './winExec'

// The Shell entry's stored command, see shellBinary.ts.
export const SHELL_COMMAND = 'zsh'

// Inherited by everything the Shell tab runs. `lgrassd hook` ends any agent session that carries it without
// LGRASS_TAB_ID, which catches every way past the shims: a full path, `command claude` after an rc reorder, npx.
export const SHELL_TAB_ENV = 'LGRASS_SHELL_TAB'

// zsh reads its startup files from ZDOTDIR, so the tab points it at ours and passes the user's own along in this.
const USER_ZDOTDIR_ENV = 'LGRASS_USER_ZDOTDIR'

// An agent started inside a Shell tab has no tab id, so lemongrass cannot route, restore or orient it.
const BLOCKED_AGENTS: Record<string, string> = { claude: 'Claude', codex: 'Codex' }

export interface ShellTabLaunch {
  args: string[]
  env: Env
}

// No apostrophes, so the text sits inside any shell's quotes unchanged.
function message(label: string): string {
  return `lemongrass: ${label} does not run inside a Shell tab, because lemongrass cannot track it there. Open a ${label} tab from New shell instead.`
}

function sq(value: string): string {
  return `'${value.replace(/'/g, `'\\''`)}'`
}

function fishQuote(value: string): string {
  return `'${value.replace(/\\/g, '\\\\').replace(/'/g, "\\'")}'`
}

function psQuote(value: string): string {
  return `'${value.replace(/'/g, "''")}'`
}

// Run after the user's own rc files, so an rc that puts the real binary back in front of PATH does not win. A
// function also outlives later PATH changes, and the PATH prefix catches `command claude`.
function posixInit(dir: string, rehash: string): string {
  const lines = [
    `unalias ${Object.keys(BLOCKED_AGENTS).join(' ')} 2>/dev/null`,
    `PATH=${sq(dir)}:"$PATH"`,
    ...Object.keys(BLOCKED_AGENTS).map((name) => `${name}() { ${sq(join(dir, name))} "$@"; }`),
    rehash
  ]
  return lines.join('\n') + '\n'
}

function shellFiles(dir: string, platform: NodeJS.Platform): Record<string, string> {
  const files: Record<string, string> = {}
  for (const [name, label] of Object.entries(BLOCKED_AGENTS)) {
    if (platform === 'win32') {
      files[`${name}.cmd`] = `@echo off\r\necho ${message(label)} 1>&2\r\nexit /b 1\r\n`
    } else {
      files[name] = `#!/bin/sh\necho "${message(label)}" >&2\nexit 1\n`
    }
  }
  if (platform === 'win32') return files
  files['bashrc'] = `[ -f "$HOME/.bashrc" ] && . "$HOME/.bashrc"\n${posixInit(dir, 'hash -r')}`
  // zsh reads .zshenv then .zshrc from ZDOTDIR. The user's .zshenv runs with their own ZDOTDIR and may change it,
  // and ours is restored afterwards so zsh still reads our .zshrc, which hands ZDOTDIR back before their .zshrc.
  files['zsh/.zshenv'] = [
    `__lg_self="$ZDOTDIR"`,
    `ZDOTDIR="\${${USER_ZDOTDIR_ENV}:-$HOME}"`,
    `[ -f "$ZDOTDIR/.zshenv" ] && . "$ZDOTDIR/.zshenv"`,
    `${USER_ZDOTDIR_ENV}="$ZDOTDIR"`,
    `ZDOTDIR="$__lg_self"`,
    `unset __lg_self`,
    ''
  ].join('\n')
  files['zsh/.zshrc'] = [
    `ZDOTDIR="$${USER_ZDOTDIR_ENV}"`,
    `unset ${USER_ZDOTDIR_ENV}`,
    `[ -f "$ZDOTDIR/.zshrc" ] && . "$ZDOTDIR/.zshrc"`,
    posixInit(dir, 'rehash')
  ].join('\n')
  files['init.fish'] =
    [
      `functions -e ${Object.keys(BLOCKED_AGENTS).join(' ')}`,
      `set -gx PATH ${fishQuote(dir)} $PATH`,
      ...Object.keys(BLOCKED_AGENTS).map(
        (name) => `function ${name}; ${fishQuote(join(dir, name))} $argv; end`
      )
    ].join('\n') + '\n'
  return files
}

// Rewritten on every Shell spawn, so an app update replaces older files. A write failure leaves the hook guard.
export function writeShellShims(
  home: string,
  platform: NodeJS.Platform = process.platform
): string | undefined {
  const dir = join(home, '.lemongrass', 'shellbin')
  try {
    mkdirSync(join(dir, 'zsh'), { recursive: true })
    for (const [name, body] of Object.entries(shellFiles(dir, platform))) {
      writeFileSync(join(dir, name), body, { mode: 0o755 })
    }
    return dir
  } catch {
    return undefined
  }
}

// Windows keeps the variable as Path after withPath, so the key is matched without case. A tab id inherited from
// the app's own environment would make the hook take an agent in this tab for a real agent tab, so it is dropped.
export function shellTabEnv(env: Env, shimDir: string | undefined): Env {
  const out: Env = { ...env, [SHELL_TAB_ENV]: '1' }
  delete out.LGRASS_TAB_ID
  if (!shimDir) return out
  const key = Object.keys(out).find((k) => k.toLowerCase() === 'path') ?? 'PATH'
  const current = out[key]
  out[key] = current ? `${shimDir}${delimiter}${current}` : shimDir
  return out
}

// How each shell is started so the init runs after the user's own startup files. Any other shell, cmd.exe included,
// keeps only the PATH shims and the hook guard.
export function shellInit(file: string, dir: string, env: Env): ShellTabLaunch {
  const name = basename(file.replace(/\\/g, '/'))
    .toLowerCase()
    .replace(/\.exe$/, '')
  if (name === 'bash') return { args: ['--rcfile', join(dir, 'bashrc')], env: {} }
  if (name === 'zsh') {
    const user = env.ZDOTDIR ? { [USER_ZDOTDIR_ENV]: env.ZDOTDIR } : {}
    return { args: [], env: { ZDOTDIR: join(dir, 'zsh'), ...user } }
  }
  if (name === 'fish')
    return { args: ['--init-command', `source ${fishQuote(join(dir, 'init.fish'))}`], env: {} }
  if (name === 'pwsh' || name === 'powershell') {
    const script = [
      `$env:Path = ${psQuote(dir + ';')} + $env:Path`,
      ...Object.keys(BLOCKED_AGENTS).flatMap((agent) => [
        `Remove-Item Alias:${agent} -ErrorAction SilentlyContinue`,
        `function global:${agent} { & ${psQuote(`${dir}\\${agent}.cmd`)} @args }`
      ])
    ].join('; ')
    return { args: ['-NoExit', '-Command', script], env: {} }
  }
  return { args: [], env: {} }
}

export function shellTabLaunch(
  file: string,
  args: string[],
  env: Env,
  home: string,
  platform: NodeJS.Platform = process.platform
): ShellTabLaunch {
  const dir = writeShellShims(home, platform)
  const tabEnv = shellTabEnv(env, dir)
  if (!dir) return { args, env: tabEnv }
  const init = shellInit(file, dir, env)
  return { args: [...init.args, ...args], env: { ...tabEnv, ...init.env } }
}
