// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { TabRecency } from '../../preload/types'

export interface OrderTab {
  id: string
  kind: string
}

// A document tab nobody has had on screen for this long is offered for closing.
export const STALE_AFTER_MS = 24 * 60 * 60 * 1000

function shellTime(tab: OrderTab, recency: Record<string, TabRecency>): number {
  const r = recency[tab.id]
  return Math.max(r?.activeAt ?? 0, r?.seenAt ?? 0)
}

function seenTime(tab: OrderTab, recency: Record<string, TabRecency>): number {
  return recency[tab.id]?.seenAt ?? 0
}

function newestFirst<T extends OrderTab>(tabs: T[], time: (tab: T) => number): T[] {
  return tabs
    .map((tab, index) => ({ tab, index, at: time(tab) }))
    .sort((a, b) => b.at - a.at || a.index - b.index)
    .map((entry) => entry.tab)
}

// The order Reorganize gives a pane: shells by their latest activity, each followed by the other
// tabs of the task it works, then every other tab by when it was last on screen. `taskOf` names a
// shell's task and a document's task; when two shells work one task, its tabs follow the more
// recent shell. Ties keep the pane's current order, so a second Reorganize changes nothing.
export function reorganizedTabs<T extends OrderTab>(
  tabs: T[],
  recency: Record<string, TabRecency>,
  taskOf: (tab: T) => string | null = () => null
): T[] {
  const shells = newestFirst(
    tabs.filter((tab) => tab.kind === 'shell'),
    (tab) => shellTime(tab, recency)
  )
  const rest = newestFirst(
    tabs.filter((tab) => tab.kind !== 'shell'),
    (tab) => seenTime(tab, recency)
  )
  const shellOfTask = new Map<string, string>()
  for (const shell of shells) {
    const task = taskOf(shell)
    if (task && !shellOfTask.has(task)) shellOfTask.set(task, shell.id)
  }
  const owned = new Map<string, T[]>()
  const loose: T[] = []
  for (const tab of rest) {
    const task = taskOf(tab)
    const owner = task ? shellOfTask.get(task) : undefined
    if (owner) owned.set(owner, [...(owned.get(owner) ?? []), tab])
    else loose.push(tab)
  }
  return [...shells.flatMap((shell) => [shell, ...(owned.get(shell.id) ?? [])]), ...loose]
}

// The tabs of a pane that `isDocument` accepts and that have been off screen longer than
// STALE_AFTER_MS. The pane's active tab is on screen, so it is never stale.
export function staleTabs<T extends OrderTab>(
  tabs: T[],
  activeTabId: string | null,
  recency: Record<string, TabRecency>,
  now: number,
  isDocument: (tab: T) => boolean
): T[] {
  return tabs.filter(
    (tab) =>
      tab.id !== activeTabId &&
      isDocument(tab) &&
      now - (recency[tab.id]?.seenAt ?? now) > STALE_AFTER_MS
  )
}
