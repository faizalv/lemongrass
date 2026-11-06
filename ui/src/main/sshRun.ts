// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { randomUUID } from 'node:crypto'
import { accessSync, constants } from 'node:fs'
import { homedir } from 'node:os'
import { delimiter, join } from 'node:path'
import type { WebContents } from 'electron'
import * as pty from 'node-pty'
import type { SSHRunRequest } from '../preload/types'
import { findKey } from './sshKeys'

// ssh-add asks for a key's passphrase itself, so it runs in a terminal the human types into,
// inside the pane. The renderer names only a key; the command line is built here from a key the
// listing shows.

interface Run {
  proc: pty.IPty
}

const runs = new Map<string, Run>()

function onPath(binary: string): string | null {
  for (const dir of (process.env.PATH ?? '').split(delimiter)) {
    if (!dir) continue
    const path = join(dir, binary)
    try {
      accessSync(path, constants.X_OK)
      return path
    } catch {
      // Not in this directory.
    }
  }
  return null
}

function commandFor(req: SSHRunRequest): { file: string; args: string[] } {
  const key = findKey(req.keyPath)
  if (!key) throw new Error('ssh: no such key in the ssh folder.')
  const file = onPath('ssh-add')
  if (!file) throw new Error('ssh: ssh-add was not found on PATH.')
  if (!key.hasPrivate) throw new Error('ssh: the key has no private file to add.')
  return { file, args: [key.privatePath] }
}

function send(sender: WebContents, channel: string, payload: unknown): void {
  if (!sender.isDestroyed()) sender.send(channel, payload)
}

export function startRun(
  sender: WebContents,
  req: SSHRunRequest,
  cols: number,
  rows: number
): string {
  const { file, args } = commandFor(req)
  const runId = randomUUID()
  const proc = pty.spawn(file, args, {
    name: 'xterm-256color',
    cols: Math.max(20, Math.floor(cols)),
    rows: Math.max(4, Math.floor(rows)),
    cwd: homedir(),
    env: process.env as Record<string, string>
  })
  proc.onData((data) => send(sender, 'ssh:run-data', { runId, data }))
  proc.onExit(({ exitCode }) => {
    runs.delete(runId)
    send(sender, 'ssh:run-exit', { runId, exitCode })
  })
  runs.set(runId, { proc })
  return runId
}

export function writeRun(runId: string, data: string): void {
  runs.get(runId)?.proc.write(data)
}

export function resizeRun(runId: string, cols: number, rows: number): void {
  runs.get(runId)?.proc.resize(Math.max(20, Math.floor(cols)), Math.max(4, Math.floor(rows)))
}

export function killRun(runId: string): void {
  runs.get(runId)?.proc.kill()
  runs.delete(runId)
}

export function killAllRuns(): void {
  for (const run of runs.values()) run.proc.kill()
  runs.clear()
}
