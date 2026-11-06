// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { execFile } from 'node:child_process'
import { existsSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'
import type { ResolvedSSHHost, SSHHostKeys, SSHStoredHostKey } from '../preload/types'
import { knownHostsLookup, parseKnownHostsMatches } from './sshText'

// A host's stored key is found and removed with ssh-keygen's own -F and -R, which also handle
// hashed known_hosts entries. The value follows its flag as one argument, so even a name starting
// with a dash is read as the name.

const KEYGEN_TIMEOUT_MS = 5_000

function knownHostsFiles(resolved: ResolvedSSHHost): string[] {
  const home = homedir()
  const files = resolved.knownHostsFiles.length
    ? resolved.knownHostsFiles
    : [join(home, '.ssh', 'known_hosts')]
  return files.map((f) => (f.startsWith('~/') ? join(home, f.slice(2)) : f)).filter(existsSync)
}

function keygen(args: string[]): Promise<{ code: number; stdout: string; stderr: string }> {
  return new Promise((resolve) => {
    execFile(
      'ssh-keygen',
      args,
      { timeout: KEYGEN_TIMEOUT_MS, encoding: 'utf8', windowsHide: true },
      (err, stdout, stderr) => {
        const code = err ? Number((err as { code?: unknown }).code ?? 1) || 1 : 0
        resolve({ code, stdout, stderr })
      }
    )
  })
}

function checkLookup(lookup: string): void {
  if (!lookup || /[\s\0]/.test(lookup)) throw new Error('ssh: the host has no usable address.')
}

export async function storedHostKeys(resolved: ResolvedSSHHost): Promise<SSHHostKeys> {
  const lookup = knownHostsLookup(resolved)
  checkLookup(lookup)
  const keys: SSHStoredHostKey[] = []
  for (const file of knownHostsFiles(resolved)) {
    // Exit 1 means no entry in this file.
    const { stdout } = await keygen(['-F', lookup, '-f', file])
    keys.push(...parseKnownHostsMatches(stdout, file))
  }
  return { lookup, keys }
}

// ssh-keygen -R keeps the previous file as <file>.old.
export async function forgetHostKey(resolved: ResolvedSSHHost): Promise<void> {
  const lookup = knownHostsLookup(resolved)
  checkLookup(lookup)
  for (const file of knownHostsFiles(resolved)) {
    const { code, stderr } = await keygen(['-R', lookup, '-f', file])
    if (code !== 0 && !/not found/i.test(stderr)) {
      throw new Error(`ssh: ${stderr.trim() || 'ssh-keygen could not update ' + file}`)
    }
  }
}
