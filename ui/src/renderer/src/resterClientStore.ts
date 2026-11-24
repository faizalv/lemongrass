// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive, watch } from 'vue'
import { plainCopy } from './components/connector/rester/resterDraft'
import { withBodyDefaults } from './resterClientCall'
import { DEFAULT_RUN_LIMIT, validRunLimit } from '../../preload/resterHistory'
import type {
  ResterClientRequest,
  ResterClientRun,
  ResterClientRunSummary
} from '../../preload/types'

const SAVE_MS = 400

// The one saved requests list and the histories opened so far. Every project's Rester Client tab
// reads this module, so two tabs edit the same list instead of overwriting each other's saves.
// Histories hold summaries only: a response body lives in the tab showing it and is read again
// by id after the tab remounts.
export const resterClientStore = reactive({
  requests: [] as ResterClientRequest[],
  loaded: false,
  histories: {} as Record<string, ResterClientRunSummary[]>
})

let runLimit: number | null = null

export async function loadRunLimit(): Promise<number> {
  if (runLimit === null) {
    const saved = (await window.api.uiState.load()).resterClientRunLimit
    runLimit = validRunLimit(saved) ? saved : DEFAULT_RUN_LIMIT
  }
  return runLimit
}

// Applies from the next send; a shorter limit trims a request's history when it next records.
export function setRunLimit(runs: number): void {
  if (!validRunLimit(runs)) return
  runLimit = runs
  window.api.uiState.save({ resterClientRunLimit: runs })
}

let loading: Promise<void> | null = null
let saveTimer: ReturnType<typeof setTimeout> | null = null

export function loadResterClient(): Promise<void> {
  loading ??= window.api.resterClient.load().then((requests) => {
    resterClientStore.requests = requests.map(withBodyDefaults)
    resterClientStore.loaded = true
  })
  return loading
}

function saveNow(): void {
  saveTimer = null
  window.api.resterClient.save(plainCopy(resterClientStore.requests))
}

watch(
  () => resterClientStore.requests,
  () => {
    if (!resterClientStore.loaded) return
    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = setTimeout(saveNow, SAVE_MS)
  },
  { deep: true }
)

window.addEventListener('beforeunload', () => {
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveNow()
  }
})

export async function loadHistory(requestId: string): Promise<void> {
  if (resterClientStore.histories[requestId]) return
  resterClientStore.histories[requestId] = await window.api.resterClient.history(requestId)
}

export async function recordRun(requestId: string, run: ResterClientRun): Promise<void> {
  resterClientStore.histories[requestId] = await window.api.resterClient.recordRun(
    requestId,
    plainCopy(run),
    await loadRunLimit()
  )
}

export function loadRun(requestId: string, runId: string): Promise<ResterClientRun | null> {
  return window.api.resterClient.run(requestId, runId)
}

export async function deleteRun(requestId: string, runId: string): Promise<void> {
  resterClientStore.histories[requestId] = await window.api.resterClient.deleteRun(requestId, runId)
}

export async function clearHistory(requestId: string): Promise<void> {
  await window.api.resterClient.clearHistory(requestId)
  resterClientStore.histories[requestId] = []
}

export async function removeRequest(requestId: string): Promise<void> {
  const index = resterClientStore.requests.findIndex((r) => r.id === requestId)
  if (index >= 0) resterClientStore.requests.splice(index, 1)
  delete resterClientStore.histories[requestId]
  await window.api.resterClient.clearHistory(requestId)
}
