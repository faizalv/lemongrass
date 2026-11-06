// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { basename, dirname, isAbsolute, join } from 'node:path'
import type { SSHHostFields, SSHIncludeState } from '../preload/types'

// The same pattern as ALIAS_PATTERN in sshHosts.ts, repeated so this file has no runtime import and
// runs under node --test.
const ALIAS_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]*$/

// The SSH pane never rewrites a file the human wrote. Its hosts live in one file it owns whole,
// pulled into the human's config by a single Include line, which must come before any Host or Match
// block: an Include inside a block applies only when that block matches.

export const MANAGED_RELATIVE = 'config.d/lemongrass.conf'
export const INCLUDE_LINE = `Include ${MANAGED_RELATIVE}`
const HEADER = '# Written by the lemongrass SSH pane. Edit these hosts there.'

export interface ManagedHost {
  fields: SSHHostFields
  // Lines of the block the pane does not edit, kept as they are so a hand addition survives a save.
  extra: string[]
}

export interface ManagedConfig {
  preamble: string[]
  hosts: ManagedHost[]
}

export function managedPath(home: string): string {
  return join(home, '.ssh', MANAGED_RELATIVE)
}

function unquote(value: string): string {
  const v = value.trim()
  return v.length >= 2 && v.startsWith('"') && v.endsWith('"') ? v.slice(1, -1) : v
}

function expandHome(path: string, home: string): string {
  return path.startsWith('~/') ? join(home, path.slice(2)) : path
}

function collapseHome(path: string, home: string): string {
  return path.startsWith(home + '/') ? '~/' + path.slice(home.length + 1) : path
}

function emptyFields(alias: string): SSHHostFields {
  return {
    alias,
    hostName: '',
    user: '',
    port: '',
    identityFile: '',
    proxyJump: '',
    identitiesOnly: false
  }
}

export function parseManagedConfig(text: string, home: string): ManagedConfig {
  const config: ManagedConfig = { preamble: [], hosts: [] }
  let current: ManagedHost | null = null
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim()
    const m = /^(\S+?)(?:\s*=\s*|\s+)(.*)$/.exec(line)
    const keyword = m?.[1].toLowerCase() ?? ''
    const value = m ? unquote(m[2]) : ''
    if (keyword === 'host') {
      current = { fields: emptyFields(value), extra: [] }
      config.hosts.push(current)
      continue
    }
    if (!current) {
      if (line && line !== HEADER) config.preamble.push(raw)
      continue
    }
    if (!line) continue
    const f = current.fields
    switch (keyword) {
      case 'hostname':
        f.hostName = value
        break
      case 'user':
        f.user = value
        break
      case 'port':
        f.port = value
        break
      case 'identityfile':
        f.identityFile = expandHome(value, home)
        break
      case 'proxyjump':
        f.proxyJump = value
        break
      case 'identitiesonly':
        f.identitiesOnly = value.toLowerCase() === 'yes'
        break
      default:
        current.extra.push(line)
    }
  }
  return config
}

function quoteIfSpaced(value: string): string {
  return /\s/.test(value) ? `"${value}"` : value
}

export function renderManagedConfig(config: ManagedConfig, home: string): string {
  const out: string[] = [HEADER, '']
  if (config.preamble.length) out.push(...config.preamble, '')
  for (const { fields: f, extra } of config.hosts) {
    out.push(`Host ${f.alias}`)
    if (f.hostName) out.push(`  HostName ${f.hostName}`)
    if (f.user) out.push(`  User ${f.user}`)
    if (f.port) out.push(`  Port ${f.port}`)
    if (f.identityFile)
      out.push(`  IdentityFile ${quoteIfSpaced(collapseHome(f.identityFile, home))}`)
    if (f.identitiesOnly) out.push('  IdentitiesOnly yes')
    if (f.proxyJump) out.push(`  ProxyJump ${f.proxyJump}`)
    for (const line of extra) out.push(`  ${line}`)
    out.push('')
  }
  return out.join('\n')
}

// Every value reaches ssh as config text, so anything that could end the line, start a quoted run
// or read as another keyword is refused rather than escaped.
export function validateHostFields(f: SSHHostFields): string | null {
  if (!ALIAS_PATTERN.test(f.alias)) {
    return 'The name may use letters, digits, dots, dashes and underscores, and starts with a letter or digit.'
  }
  if (!f.hostName) return 'The address is required.'
  if (!/^[A-Za-z0-9._:%[\]][A-Za-z0-9._:%[\]-]*$/.test(f.hostName)) {
    return 'The address must be a host name or an IP address.'
  }
  if (f.user && !/^[A-Za-z0-9._][A-Za-z0-9._@-]*$/.test(f.user)) {
    return 'The user name has characters ssh does not accept here.'
  }
  if (f.port) {
    const port = Number(f.port)
    if (!/^\d+$/.test(f.port) || port < 1 || port > 65535) return 'The port must be 1 to 65535.'
  }
  if (f.identityFile && (!isAbsolute(f.identityFile) || /["\r\n]/.test(f.identityFile))) {
    return 'Pick the key from the list.'
  }
  if (f.proxyJump && !/^[A-Za-z0-9._@[\]][A-Za-z0-9._@:,[\]-]*$/.test(f.proxyJump)) {
    return 'The jump host must be a host name from the list.'
  }
  return null
}

// Whether an Include argument names the managed file, directly or through a glob in its last
// segment such as `config.d/*`. The file may not exist yet, so the glob is matched by name.
function includesManaged(arg: string, home: string): boolean {
  const sshDir = join(home, '.ssh')
  let path = arg.startsWith('~/') ? join(home, arg.slice(2)) : arg
  if (!isAbsolute(path)) path = join(sshDir, path)
  const target = managedPath(home)
  if (path === target) return true
  if (dirname(path) !== dirname(target)) return false
  const name = basename(path)
  if (!/[*?]/.test(name)) return false
  const re = new RegExp(
    '^' +
      name
        .replace(/[.+^${}()|[\]\\]/g, '\\$&')
        .replace(/\*/g, '.*')
        .replace(/\?/g, '.') +
      '$'
  )
  return re.test(basename(target))
}

export function includeState(configText: string | null, home: string): SSHIncludeState {
  if (configText === null) return 'missing'
  let inBlock = false
  let insideBlock = false
  for (const raw of configText.split(/\r?\n/)) {
    const line = raw.trim()
    if (!line || line.startsWith('#')) continue
    const m = /^(\S+?)(?:\s*=\s*|\s+)(.*)$/.exec(line)
    if (!m) continue
    const keyword = m[1].toLowerCase()
    if (keyword === 'host' || keyword === 'match') {
      inBlock = true
      continue
    }
    if (keyword !== 'include') continue
    const args = m[2].match(/"[^"]*"|\S+/g) ?? []
    if (!args.some((arg) => includesManaged(unquote(arg), home))) continue
    if (!inBlock) return 'included'
    insideBlock = true
  }
  return insideBlock ? 'insideBlock' : 'missing'
}

export function withIncludeLine(configText: string | null): string {
  const rest = configText ?? ''
  return `${INCLUDE_LINE}\n\n${rest}`
}
