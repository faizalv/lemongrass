// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { execFile } from 'node:child_process'
import { existsSync, mkdirSync, readdirSync, readFileSync, statSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'
import type { SSHCreateKeyRequest, SSHKey, SSHKeyList } from '../preload/types'
import { fingerprint, parsePublicKeyLine } from './sshText'

// Keys are listed from their .pub files and fingerprinted here, so opening the pane starts no
// process per key. The private file is only checked for existence, never read.

const AGENT_TIMEOUT_MS = 3_000
const KEYGEN_TIMEOUT_MS = 10_000
const KEY_NAME_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/

export function sshDir(): string {
  return join(homedir(), '.ssh')
}

// `ssh-add -L` prints the agent's public keys; exit 1 is an agent with none, 2 is no agent.
function agentBlobs(): Promise<Set<string> | null> {
  return new Promise((resolve) => {
    execFile(
      'ssh-add',
      ['-L'],
      { timeout: AGENT_TIMEOUT_MS, encoding: 'utf8', windowsHide: true },
      (err, stdout) => {
        const code = (err as { code?: unknown } | null)?.code
        if (err && code !== 1) return resolve(null)
        const blobs = new Set<string>()
        for (const line of stdout.split(/\r?\n/)) {
          const parsed = parsePublicKeyLine(line)
          if (parsed) blobs.add(parsed.blob)
        }
        resolve(blobs)
      }
    )
  })
}

interface ReadKey extends SSHKey {
  blob: string
}

function readKeys(): ReadKey[] {
  const dir = sshDir()
  let names: string[]
  try {
    names = readdirSync(dir)
  } catch {
    return []
  }
  const keys: ReadKey[] = []
  for (const entry of names.filter((n) => n.endsWith('.pub')).sort()) {
    const publicPath = join(dir, entry)
    let text: string
    try {
      if (!statSync(publicPath).isFile()) continue
      text = readFileSync(publicPath, 'utf8')
    } catch {
      continue
    }
    const parsed = parsePublicKeyLine(text)
    if (!parsed) continue
    const name = entry.slice(0, -'.pub'.length)
    const privatePath = join(dir, name)
    keys.push({
      name,
      privatePath,
      publicPath,
      hasPrivate: existsSync(privatePath),
      type: parsed.type,
      comment: parsed.comment,
      fingerprint: fingerprint(parsed.blob),
      inAgent: false,
      blob: parsed.blob
    })
  }
  return keys
}

export async function listKeys(): Promise<SSHKeyList> {
  const agent = await agentBlobs()
  const keys = readKeys().map(({ blob, ...key }) => ({ ...key, inAgent: !!agent?.has(blob) }))
  return { keys, agent: agent !== null }
}

// A renderer request names a key by path; only a pair the listing would show is accepted.
export function findKey(path: string): SSHKey | undefined {
  return readKeys().find((key) => key.privatePath === path || key.publicPath === path)
}

export function readPublicKey(path: string): string {
  const key = findKey(path)
  if (!key) throw new Error('ssh: no such key in the ssh folder.')
  return readFileSync(key.publicPath, 'utf8').trim()
}

// lgrassd writes the pair, since ssh-keygen would need the passphrase on its command line.
export function createKey(lgrassdPath: string | null, req: SSHCreateKeyRequest): Promise<SSHKey> {
  if (!lgrassdPath) return Promise.reject(new Error('ssh: lgrassd is not installed.'))
  if (!KEY_NAME_PATTERN.test(req.name) || req.name.endsWith('.pub')) {
    return Promise.reject(
      new Error(
        'The file name may use letters, digits, dots, dashes and underscores, and starts with a letter or digit.'
      )
    )
  }
  if (/[\r\n]/.test(req.comment)) return Promise.reject(new Error('The comment must be one line.'))
  const dir = sshDir()
  mkdirSync(dir, { recursive: true, mode: 0o700 })
  const path = join(dir, req.name)
  if (existsSync(path) || existsSync(path + '.pub')) {
    return Promise.reject(new Error(`A key named ${req.name} already exists.`))
  }
  const args = ['ssh', 'keygen', path, ...(req.comment ? ['--comment', req.comment] : [])]
  return new Promise((resolve, reject) => {
    const child = execFile(
      lgrassdPath,
      args,
      { timeout: KEYGEN_TIMEOUT_MS, encoding: 'utf8', windowsHide: true },
      (err, _stdout, stderr) => {
        if (err) {
          reject(new Error(stderr.replace(/^error: /, '').trim() || err.message))
          return
        }
        const key = findKey(path)
        if (key) resolve(key)
        else reject(new Error('ssh: the key was written but could not be read back.'))
      }
    )
    child.stdin?.end(req.passphrase)
  })
}
