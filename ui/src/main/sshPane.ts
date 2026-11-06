// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { execFile } from 'node:child_process'
import {
  mkdirSync,
  readFileSync,
  realpathSync,
  renameSync,
  statSync,
  unlinkSync,
  writeFileSync
} from 'node:fs'
import { homedir } from 'node:os'
import { basename, dirname, join } from 'node:path'
import { clipboard, ipcMain } from 'electron'
import type {
  SSHCreateKeyRequest,
  SSHHostEntry,
  SSHHostFields,
  SSHHostKeys,
  SSHHostList,
  SSHHostBlock,
  SSHRunRequest
} from '../preload/types'
import {
  INCLUDE_LINE,
  includeState,
  managedPath,
  parseManagedConfig,
  renderManagedConfig,
  validateHostFields,
  withIncludeLine
} from './sshConfigFile'
import { listSSHHostSources, resolveSSHHost } from './sshHosts'
import { createKey, findKey, listKeys, readPublicKey, sshDir } from './sshKeys'
import { forgetHostKey, storedHostKeys } from './sshKnownHosts'
import { hostBlockRange } from './sshText'
import { killRun, resizeRun, startRun, writeRun } from './sshRun'
import { registerSSHSessionHandlers } from './sshSession'
import { terminalArgs } from './sshTerminal'

// The SSH pane is the human's own tool and runs here in Electron main, never through the vault
// daemon, so no agent channel can reach it.

const CHECK_TIMEOUT_MS = 3_000

// With no IdentityFile set, `ssh -G` prints OpenSSH's whole default list, which says nothing about
// the key a host was set up with, so such a host is not shown as using any key.
const DEFAULT_IDENTITIES = new Set([
  'id_rsa',
  'id_ecdsa',
  'id_ecdsa_sk',
  'id_ed25519',
  'id_ed25519_sk',
  'id_xmss',
  'id_dsa'
])

function explicitKeyPaths(identityFiles: string[], home: string): string[] {
  const expanded = identityFiles.map((f) => (f.startsWith('~/') ? join(home, f.slice(2)) : f))
  const isDefaultList =
    expanded.length >= 4 &&
    expanded.every((f) => dirname(f) === sshDir() && DEFAULT_IDENTITIES.has(basename(f)))
  return isDefaultList ? [] : expanded
}

function readOrNull(path: string): string | null {
  try {
    return readFileSync(path, 'utf8')
  } catch {
    return null
  }
}

function configPath(): string {
  return join(sshDir(), 'config')
}

// A dotfiles manager often keeps ~/.ssh/config as a symlink; writing through to its target keeps
// the link instead of replacing it with a plain file.
function writeTarget(path: string): string {
  try {
    return realpathSync(path)
  } catch {
    return path
  }
}

function writeReplacing(path: string, text: string, fallbackMode: number): void {
  const target = writeTarget(path)
  mkdirSync(dirname(target), { recursive: true, mode: 0o700 })
  let mode = fallbackMode
  try {
    mode = statSync(target).mode & 0o777
  } catch {
    // New file.
  }
  const temp = `${target}.lemongrass-tmp`
  writeFileSync(temp, text, { mode })
  renameSync(temp, target)
}

async function listHosts(): Promise<SSHHostList> {
  const home = homedir()
  const managed = managedPath(home)
  const ownFields = new Map(
    parseManagedConfig(readOrNull(managed) ?? '', home).hosts.map((h) => [h.fields.alias, h.fields])
  )
  const sources = listSSHHostSources()
  const seen = new Set(sources.map((s) => s.alias))
  // A host in the managed file shows even while the Include line is missing.
  for (const alias of ownFields.keys()) {
    if (!seen.has(alias)) sources.push({ alias, file: managed })
  }
  const hosts = await Promise.all(
    sources.map(async ({ alias, file }): Promise<SSHHostEntry> => {
      const entry: SSHHostEntry = { alias, file, managed: file === managed, keyPaths: [] }
      if (entry.managed) entry.fields = ownFields.get(alias)
      try {
        entry.resolved = await resolveSSHHost(alias)
        entry.keyPaths = explicitKeyPaths(entry.resolved.identityFiles, home)
      } catch (err) {
        entry.resolveError = err instanceof Error ? err.message : String(err)
      }
      return entry
    })
  )
  return {
    hosts,
    include: includeState(readOrNull(configPath()), home),
    configFile: configPath(),
    managedFile: managed,
    includeLine: INCLUDE_LINE,
    home
  }
}

// `ssh -G -F <file>` parses only the managed file, so a bad line is caught even before the Include
// line is in place.
function checkManagedFile(path: string, alias: string): Promise<void> {
  return new Promise((resolve, reject) => {
    execFile(
      'ssh',
      ['-G', '-F', path, '--', alias],
      { timeout: CHECK_TIMEOUT_MS, encoding: 'utf8', windowsHide: true },
      (err, _stdout, stderr) => (err ? reject(new Error(stderr.trim() || err.message)) : resolve())
    )
  })
}

async function saveHost(original: string | null, fields: SSHHostFields): Promise<void> {
  const problem = validateHostFields(fields)
  if (problem) throw new Error(problem)
  const home = homedir()
  const path = managedPath(home)
  if (fields.proxyJump === fields.alias) throw new Error('A host cannot jump through itself.')
  if (fields.identityFile && findKey(fields.identityFile)?.privatePath !== fields.identityFile) {
    throw new Error('Pick the key from the list.')
  }
  const elsewhere = listSSHHostSources().find((s) => s.alias === fields.alias && s.file !== path)
  if (elsewhere)
    throw new Error(`A host named ${fields.alias} already exists in ${elsewhere.file}.`)

  const before = readOrNull(path)
  const config = parseManagedConfig(before ?? '', home)
  const clash = config.hosts.find(
    (h) => h.fields.alias === fields.alias && h.fields.alias !== original
  )
  if (clash) throw new Error(`A host named ${fields.alias} already exists.`)
  const index = original === null ? -1 : config.hosts.findIndex((h) => h.fields.alias === original)
  if (index >= 0) config.hosts[index] = { fields, extra: config.hosts[index].extra }
  else config.hosts.push({ fields, extra: [] })

  writeReplacing(path, renderManagedConfig(config, home), 0o600)
  try {
    await checkManagedFile(path, fields.alias)
  } catch (err) {
    if (before === null) unlinkSync(path)
    else writeReplacing(path, before, 0o600)
    throw err
  }
}

function deleteHost(alias: string): void {
  const home = homedir()
  const path = managedPath(home)
  const text = readOrNull(path)
  if (text === null) return
  const config = parseManagedConfig(text, home)
  config.hosts = config.hosts.filter((h) => h.fields.alias !== alias)
  writeReplacing(path, renderManagedConfig(config, home), 0o600)
}

function addInclude(): void {
  const path = configPath()
  const text = readOrNull(path)
  if (includeState(text, homedir()) === 'included') return
  writeReplacing(path, withIncludeLine(text), 0o600)
}

async function hostKeys(alias: string): Promise<SSHHostKeys> {
  return storedHostKeys(await resolveSSHHost(alias))
}

async function forgetKey(alias: string): Promise<void> {
  await forgetHostKey(await resolveSSHHost(alias))
}

// The renderer names a host, never a path, so only a config file the listing found is read.
function hostSource(alias: string): SSHHostBlock {
  const home = homedir()
  const managed = managedPath(home)
  const source = listSSHHostSources().find((s) => s.alias === alias)
  if (source) {
    const text = readOrNull(source.file)
    if (text === null) throw new Error(`ssh: cannot read ${source.file}.`)
    const range = hostBlockRange(text, alias)
    return { file: source.file, text, start: range?.start ?? 1, end: range?.end ?? 1 }
  }
  const text = readOrNull(managed) ?? ''
  const range = hostBlockRange(text, alias)
  if (!range) throw new Error(`ssh: no host named ${alias}.`)
  return { file: managed, text, ...range }
}

export function registerSSHPaneHandlers(lgrassdPath: string | null): void {
  ipcMain.handle('ssh:listKeys', () => listKeys())
  ipcMain.handle('ssh:createKey', (_event, req: SSHCreateKeyRequest) => createKey(lgrassdPath, req))
  ipcMain.handle('ssh:copyPublicKey', (_event, path: string) => {
    clipboard.writeText(readPublicKey(path))
  })
  ipcMain.handle('ssh:listHosts', () => listHosts())
  ipcMain.handle('ssh:saveHost', (_event, original: string | null, fields: SSHHostFields) =>
    saveHost(original, fields)
  )
  ipcMain.handle('ssh:deleteHost', (_event, alias: string) => deleteHost(alias))
  ipcMain.handle('ssh:addInclude', () => addInclude())
  ipcMain.handle('ssh:hostKeys', (_event, alias: string) => hostKeys(alias))
  ipcMain.handle('ssh:forgetHostKey', (_event, alias: string) => forgetKey(alias))
  ipcMain.handle('ssh:hostSource', (_event, alias: string) => hostSource(alias))
  ipcMain.handle('ssh:publicKey', (_event, path: string) => readPublicKey(path))
  ipcMain.handle('ssh:terminalArgs', (_event, alias: string) => terminalArgs(alias))
  ipcMain.handle('ssh:runStart', (event, req: SSHRunRequest, cols: number, rows: number) =>
    startRun(event.sender, req, cols, rows)
  )
  ipcMain.on('ssh:runInput', (_event, runId: string, data: string) => writeRun(runId, data))
  ipcMain.on('ssh:runResize', (_event, runId: string, cols: number, rows: number) =>
    resizeRun(runId, cols, rows)
  )
  ipcMain.on('ssh:runKill', (_event, runId: string) => killRun(runId))
  registerSSHSessionHandlers(lgrassdPath)
}
