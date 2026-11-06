// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { createHash } from 'node:crypto'
import type { ResolvedSSHHost, SSHStoredHostKey } from '../preload/types'

// Pure readers of OpenSSH text formats for the SSH pane, kept free of sibling imports so they run
// under node --test.

export interface PublicKeyLine {
  type: string
  blob: string
  comment: string
}

export function parsePublicKeyLine(text: string): PublicKeyLine | null {
  const line = text.split(/\r?\n/).find((l) => l.trim() && !l.trim().startsWith('#'))
  if (!line) return null
  const m = /^((?:ssh|ecdsa|sk)-[A-Za-z0-9@.-]+)\s+([A-Za-z0-9+/]{16,}={0,2})(?:\s+(.*))?$/.exec(
    line.trim()
  )
  if (!m) return null
  return { type: m[1], blob: m[2], comment: m[3]?.trim() ?? '' }
}

// OpenSSH's SHA256 fingerprint: unpadded base64 of the digest of the decoded key blob.
export function fingerprint(blob: string): string {
  const digest = createHash('sha256').update(Buffer.from(blob, 'base64')).digest('base64')
  return 'SHA256:' + digest.replace(/=+$/, '')
}

// How ssh names a host in known_hosts: the HostKeyAlias if set, else the HostName, in brackets
// with the port when it is not 22.
export function knownHostsLookup(resolved: ResolvedSSHHost): string {
  const name = resolved.hostKeyAlias || resolved.hostname
  const port = resolved.port || '22'
  return port === '22' ? name : `[${name}]:${port}`
}

// The 1-based line range of the first Host block naming alias: its Host line through the line
// before the next Host or Match, trailing blank lines dropped. Null when no block names it.
export function hostBlockRange(text: string, alias: string): { start: number; end: number } | null {
  const lines = text.split(/\r?\n/)
  let start = -1
  for (let i = 0; i < lines.length; i++) {
    const m = /^\s*(host|match)(?:\s*=\s*|\s+)(.*)$/i.exec(lines[i])
    if (!m) continue
    if (start >= 0) {
      let end = i
      while (end > start + 1 && !lines[end - 1].trim()) end--
      return { start: start + 1, end }
    }
    if (m[1].toLowerCase() === 'host' && m[2].split(/\s+/).includes(alias)) start = i
  }
  if (start < 0) return null
  let end = lines.length
  while (end > start + 1 && !lines[end - 1].trim()) end--
  return { start: start + 1, end }
}

// Lines of `ssh-keygen -F` output: comments, then `[@marker] <host> <type> <blob>` per match.
export function parseKnownHostsMatches(output: string, file: string): SSHStoredHostKey[] {
  const keys: SSHStoredHostKey[] = []
  for (const raw of output.split(/\r?\n/)) {
    const line = raw.trim()
    if (!line || line.startsWith('#')) continue
    const parts = line.split(/\s+/)
    const offset = parts[0].startsWith('@') ? 1 : 0
    const type = parts[offset + 1]
    const blob = parts[offset + 2]
    if (!type || !blob || !/^[A-Za-z0-9+/=]+$/.test(blob)) continue
    keys.push({ type, fingerprint: fingerprint(blob), file })
  }
  return keys
}
