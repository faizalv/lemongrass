// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import type { TabHookState } from '../../preload/types'

// Each shell tab's hook state, kept live by the tab_state push that a hook sends on every change.
// A tab with no entry is idle. The app clears every state when it loads a project and its tabs
// report afresh, so the one read at start only covers a renderer reload under running tabs.
const states = reactive<Record<string, TabHookState>>({})
const pushed = new Set<string>()
const listeners = new Set<(tabId: string, state: TabHookState) => void>()

let started = false

export function tabHookState(tabId: string): TabHookState {
  return states[tabId] ?? 'idle'
}

// The design system's activity for a tab: a prompt or question picker holds it up until the human answers.
export function tabActivity(tabId: string): 'working' | 'waiting' | null {
  const state = tabHookState(tabId)
  if (state === 'working') return 'working'
  if (state === 'prompting') return 'waiting'
  return null
}

// Called on every pushed state change, never for the seed read at start.
export function onTabState(listener: (tabId: string, state: TabHookState) => void): void {
  listeners.add(listener)
}

function set(tabId: string, state: TabHookState): void {
  if (state === 'idle') delete states[tabId]
  else states[tabId] = state
}

async function seed(): Promise<void> {
  const projects = await window.api.projects.list()
  for (const project of projects) {
    const read = await window.api.tabSessions.states(project.path).catch(() => ({}))
    for (const [tabId, busy] of Object.entries(read)) {
      // A push that arrived during the read is newer, so the read never overwrites it.
      if (!pushed.has(tabId)) set(tabId, busy.state)
    }
  }
}

export function startTabStates(): void {
  if (started) return
  started = true
  window.api.tabSessions.onStateChanged(({ tabId, state }) => {
    pushed.add(tabId)
    set(tabId, state)
    for (const listener of listeners) listener(tabId, state)
  })
  void seed().catch(() => undefined)
}
