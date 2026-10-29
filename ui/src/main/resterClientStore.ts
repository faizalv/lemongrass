// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

// Saved requests and their history for the Rester Client. Resters belong to the vault, not to a
// project, so nothing here is keyed by project: one requests file and one history file per request.

import { existsSync, mkdirSync, readFileSync, unlinkSync, writeFileSync } from 'fs'
import { join } from 'path'
import type { ResterClientRequest, ResterClientRun } from '../preload/types'

export const HISTORY_LIMIT = 50
export const HISTORY_BODY_BYTES = 256 * 1024

// A request id becomes a file name, so only the characters a UUID uses are accepted.
function historyFile(dir: string, requestId: string): string {
  if (!/^[A-Za-z0-9-]+$/.test(requestId)) throw new Error(`lgrass: invalid request id ${requestId}`)
  return join(dir, 'history', `${requestId}.json`)
}

function readJson<T>(file: string, fallback: T): T {
  if (!existsSync(file)) return fallback
  try {
    return JSON.parse(readFileSync(file, 'utf-8')) as T
  } catch {
    return fallback
  }
}

function writeJson(file: string, value: unknown): void {
  mkdirSync(join(file, '..'), { recursive: true })
  writeFileSync(file, JSON.stringify(value, null, 2))
}

// Reads the global list. The first build kept one list per project in legacyFile; on first load
// those lists are merged, without repeating an id, into the global file and legacyFile is removed.
export function loadRequests(dir: string, legacyFile: string): ResterClientRequest[] {
  const file = join(dir, 'requests.json')
  if (existsSync(file) || !existsSync(legacyFile)) return readJson<ResterClientRequest[]>(file, [])
  const legacy = readJson<Record<string, ResterClientRequest[]>>(legacyFile, {})
  const merged: ResterClientRequest[] = []
  const seen = new Set<string>()
  for (const list of Object.values(legacy)) {
    for (const request of list ?? []) {
      if (seen.has(request.id)) continue
      seen.add(request.id)
      merged.push(request)
    }
  }
  writeJson(file, merged)
  unlinkSync(legacyFile)
  return merged
}

export function saveRequests(dir: string, requests: ResterClientRequest[]): void {
  writeJson(join(dir, 'requests.json'), requests)
}

export function loadHistory(dir: string, requestId: string): ResterClientRun[] {
  return readJson<ResterClientRun[]>(historyFile(dir, requestId), [])
}

// Adds run first, cutting a long response body, keeps the newest HISTORY_LIMIT runs and returns
// the history as stored.
export function appendRun(dir: string, requestId: string, run: ResterClientRun): ResterClientRun[] {
  const stored = { ...run, bodyCut: false }
  if (stored.result && Buffer.byteLength(stored.result.body) > HISTORY_BODY_BYTES) {
    const body = Buffer.from(stored.result.body).subarray(0, HISTORY_BODY_BYTES).toString('utf-8')
    stored.result = { ...stored.result, body }
    stored.bodyCut = true
  }
  const history = [stored, ...loadHistory(dir, requestId)].slice(0, HISTORY_LIMIT)
  writeJson(historyFile(dir, requestId), history)
  return history
}

export function deleteRun(dir: string, requestId: string, runId: string): ResterClientRun[] {
  const history = loadHistory(dir, requestId).filter((run) => run.id !== runId)
  writeJson(historyFile(dir, requestId), history)
  return history
}

export function clearHistory(dir: string, requestId: string): void {
  const file = historyFile(dir, requestId)
  if (existsSync(file)) unlinkSync(file)
}
