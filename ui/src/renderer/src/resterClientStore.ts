// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { reactive, watch } from 'vue'
import { plainCopy } from './components/connector/rester/resterDraft'
import type { ResterClientRequest, ResterClientRun } from '../../preload/types'

const SAVE_MS = 400

// The one saved requests list and the histories opened so far. Every project's Rester Client tab
// reads this module, so two tabs edit the same list instead of overwriting each other's saves.
export const resterClientStore = reactive({
  requests: [] as ResterClientRequest[],
  loaded: false,
  histories: {} as Record<string, ResterClientRun[]>
})

let loading: Promise<void> | null = null
let saveTimer: ReturnType<typeof setTimeout> | null = null

export function loadResterClient(): Promise<void> {
  loading ??= window.api.resterClient.load().then((requests) => {
    resterClientStore.requests = requests
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
    plainCopy(run)
  )
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
