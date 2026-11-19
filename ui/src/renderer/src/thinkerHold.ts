// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import { startShellDetached, type ShellSpec } from './shellRegistry'
import { ThinkerHolds, thinkerCacheKey } from './thinkerStartPlan'

// Thinkers started together each miss the prompt cache and write their own copy of the system
// prompt, since a cache entry is readable only once the request that wrote it has been answered.
// So the first thinker of each cache key starts alone and the rest wait for its first tool call.

export interface ThinkerToStart {
  spec: ShellSpec
  vendor: string
  account?: string
  model?: string
}

const RELEASE_TIMEOUT_MS = 30_000
const HELD_ARGS_KEY = 'lemongrass.heldThinkerArgs'

// Held tab ids, read by the tab body and the workgroup pane.
export const heldThinkers = reactive<Record<string, true>>({})
const heldSpecs = new Map<string, ShellSpec>()

const holds = new ThinkerHolds(
  (tabId) => {
    const spec = heldSpecs.get(tabId)
    forgetHeld(tabId)
    if (spec) startShellDetached(spec)
  },
  {
    set: (fn, ms) => setTimeout(fn, ms),
    clear: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>)
  },
  RELEASE_TIMEOUT_MS
)

function forgetHeld(tabId: string): void {
  heldSpecs.delete(tabId)
  delete heldThinkers[tabId]
  forgetHeldArgs(tabId)
}

// Starts what the plan allows now and holds the rest. Shell args must already be set for every spec.
export function startThinkers(
  projectPath: string,
  thinkers: ThinkerToStart[],
  argsOf: (tabId: string) => string[]
): void {
  const specOf = new Map(thinkers.map((thinker) => [thinker.spec.id, thinker.spec]))
  const plan = holds.plan(
    projectPath,
    thinkers.map((thinker) => ({
      tabId: thinker.spec.id,
      key: thinkerCacheKey(thinker.vendor, thinker.account, thinker.model)
    }))
  )
  for (const waiting of plan.held.values()) {
    for (const tabId of waiting) {
      heldSpecs.set(tabId, specOf.get(tabId)!)
      heldThinkers[tabId] = true
      rememberHeldArgs(tabId, argsOf(tabId))
    }
  }
  for (const tabId of plan.start) startShellDetached(specOf.get(tabId)!)
}

export function noteThinkerStarted(tabId: string): void {
  holds.noteStarted(tabId)
}

// A thinker whose process exited or whose tab closed. A held tab is dropped so a release never starts a closed tab.
export function noteThinkerGone(tabId: string): void {
  if (holds.noteGone(tabId)) forgetHeld(tabId)
}

// Held args are kept across an app restart, since a held tab has no session to resume and would
// otherwise reopen without its starting prompt.
function heldArgs(): Record<string, string[]> {
  try {
    const parsed = JSON.parse(localStorage.getItem(HELD_ARGS_KEY) ?? '{}')
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

function rememberHeldArgs(tabId: string, args: string[]): void {
  localStorage.setItem(HELD_ARGS_KEY, JSON.stringify({ ...heldArgs(), [tabId]: args }))
}

function forgetHeldArgs(tabId: string): void {
  const all = heldArgs()
  if (!(tabId in all)) return
  delete all[tabId]
  localStorage.setItem(HELD_ARGS_KEY, JSON.stringify(all))
}

// The starting args of a tab that was still held when the app closed, removed once taken.
export function takeHeldArgs(tabId: string): string[] | undefined {
  const args = heldArgs()[tabId]
  if (args !== undefined) forgetHeldArgs(tabId)
  return Array.isArray(args) ? args : undefined
}
