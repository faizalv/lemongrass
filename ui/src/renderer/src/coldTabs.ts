// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import { cacheTtlKnown, isColdAndLarge } from '../../preload/cacheState'
import { ageNow } from './ageClock'
import { contextStore } from './usage'

// Both maps hold the tab's last response time (seen_at) when the event happened, so a new response
// clears them without a reset: Continue anyway lasts until the tab goes cold again, and a held
// nudge note until the agent has answered.
const continuedAt = reactive<Record<string, number>>({})
const nudgeHeldAt = reactive<Record<string, number>>({})

let started = false

function coldAt(tabId: string, nowMs: number): boolean {
  const context = contextStore.tabs[tabId]
  return isColdAndLarge(context, nowMs) && continuedAt[tabId] !== context?.seen_at
}

// Only a lifetime read from the transcript blocks the tab; an assumed one never does.
function blockingAt(tabId: string, nowMs: number): boolean {
  const context = contextStore.tabs[tabId]
  return !!context && cacheTtlKnown(context) && coldAt(tabId, nowMs)
}

// Whether the tab shows the blocking cold warning, on the shared age clock.
export function coldWarningShown(tabId: string): boolean {
  return blockingAt(tabId, ageNow.value)
}

// Whether the tab may have gone cold on the assumed lifetime: a soft notice, nothing held.
export function coldAssumed(tabId: string): boolean {
  const context = contextStore.tabs[tabId]
  return !!context && !cacheTtlKnown(context) && coldAt(tabId, ageNow.value)
}

// Whether keystrokes into the tab are held: only while its warning is on screen, so nothing the
// human types is dropped unseen. The age clock ticks every 30 seconds, so a key into a tab that
// went cold since the last tick advances it: that key passes and the warning appears at once.
export function shellInputHeld(tabId: string): boolean {
  if (blockingAt(tabId, ageNow.value)) return true
  const now = Date.now()
  if (blockingAt(tabId, now)) ageNow.value = now
  return false
}

export function continueCold(tabId: string): void {
  const context = contextStore.tabs[tabId]
  if (context) continuedAt[tabId] = context.seen_at
}

export function nudgeWaiting(tabId: string): boolean {
  const context = contextStore.tabs[tabId]
  return !!context && nudgeHeldAt[tabId] === context.seen_at
}

export function startColdTabs(): void {
  if (started) return
  started = true
  window.api.tabSessions.onNudgeHeld(({ tabId }) => {
    const context = contextStore.tabs[tabId]
    if (context) nudgeHeldAt[tabId] = context.seen_at
  })
}
