// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Saved requests and their history for the Rester Client. Resters belong to the vault, not to a
// project, so nothing here is keyed by project: one requests file, and per request a history
// folder holding an index of run summaries and one file per run.

import { existsSync, mkdirSync, readFileSync, rmSync, unlinkSync, writeFileSync } from 'fs'
import { join } from 'path'
import type {
  ResterClientRequest,
  ResterClientResult,
  ResterClientRun,
  ResterClientRunSummary
} from '../preload/types'

// Request and run ids become file names, so only the characters a UUID uses are accepted.
function plainId(id: string): string {
  if (!/^[A-Za-z0-9-]+$/.test(id)) throw new Error(`lgrass: invalid id ${id}`)
  return id
}

function historyDir(dir: string, requestId: string): string {
  return join(dir, 'history', plainId(requestId))
}

// The single file per request that history used before runs got files of their own.
function legacyHistoryFile(dir: string, requestId: string): string {
  return join(dir, 'history', `${plainId(requestId)}.json`)
}

function indexFile(dir: string, requestId: string): string {
  return join(historyDir(dir, requestId), 'index.json')
}

function runFile(dir: string, requestId: string, runId: string): string {
  return join(historyDir(dir, requestId), `${plainId(runId)}.json`)
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

function summarize(run: ResterClientRun, bytes: number): ResterClientRunSummary {
  const { result, ...rest } = run
  if (!result) return { ...rest, result: null, bytes }
  const head: Partial<ResterClientResult> = { ...result }
  delete head.body
  return { ...rest, result: head as Omit<ResterClientResult, 'body'>, bytes }
}

// Writes the run compact, since a body can be megabytes, and returns the bytes written.
function writeRun(dir: string, requestId: string, run: ResterClientRun): number {
  const text = JSON.stringify(run)
  mkdirSync(historyDir(dir, requestId), { recursive: true })
  writeFileSync(runFile(dir, requestId, run.id), text)
  return Buffer.byteLength(text)
}

function removeRunFile(dir: string, requestId: string, runId: string): void {
  const file = runFile(dir, requestId, runId)
  if (existsSync(file)) unlinkSync(file)
}

// Moves a request's history from its old single file into per-run files once.
function migrateHistory(dir: string, requestId: string): void {
  const legacy = legacyHistoryFile(dir, requestId)
  if (!existsSync(legacy) || existsSync(indexFile(dir, requestId))) return
  const runs = readJson<ResterClientRun[]>(legacy, []).filter((run) =>
    /^[A-Za-z0-9-]+$/.test(run?.id ?? '')
  )
  const summaries = runs.map((run) => summarize(run, writeRun(dir, requestId, run)))
  writeJson(indexFile(dir, requestId), summaries)
  unlinkSync(legacy)
}

// The request's runs, newest first, without their response bodies.
export function loadHistory(dir: string, requestId: string): ResterClientRunSummary[] {
  migrateHistory(dir, requestId)
  return readJson<ResterClientRunSummary[]>(indexFile(dir, requestId), [])
}

// One run in full, or null once it was trimmed or deleted.
export function loadRun(dir: string, requestId: string, runId: string): ResterClientRun | null {
  return readJson<ResterClientRun | null>(runFile(dir, requestId, runId), null)
}

// Adds run first with its body whole, then keeps older runs newest first while both runLimit and
// budgetBytes hold, the newest counted too. The newest is kept whatever its size, and the first
// older run that does not fit ends the history. The limits come from preload/resterHistory.ts.
export function appendRun(
  dir: string,
  requestId: string,
  run: ResterClientRun,
  runLimit: number,
  budgetBytes: number
): ResterClientRunSummary[] {
  const previous = loadHistory(dir, requestId)
  const stored = { ...run, bodyCut: false }
  const history = [summarize(stored, writeRun(dir, requestId, stored)), ...previous]
  let total = history[0].bytes
  let keep = 1
  while (keep < history.length && keep < runLimit && total + history[keep].bytes <= budgetBytes) {
    total += history[keep].bytes
    keep++
  }
  const kept = history.slice(0, keep)
  writeJson(indexFile(dir, requestId), kept)
  for (const dropped of history.slice(keep)) removeRunFile(dir, requestId, dropped.id)
  return kept
}

export function deleteRun(dir: string, requestId: string, runId: string): ResterClientRunSummary[] {
  const history = loadHistory(dir, requestId).filter((run) => run.id !== runId)
  writeJson(indexFile(dir, requestId), history)
  removeRunFile(dir, requestId, runId)
  return history
}

export function clearHistory(dir: string, requestId: string): void {
  rmSync(historyDir(dir, requestId), { recursive: true, force: true })
  const legacy = legacyHistoryFile(dir, requestId)
  if (existsSync(legacy)) unlinkSync(legacy)
}
