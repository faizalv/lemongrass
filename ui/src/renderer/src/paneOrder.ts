// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { TabRecency } from '../../preload/types'

export interface OrderTab {
  id: string
  kind: string
}

// A document tab nobody has had on screen for this long is offered for closing.
export const STALE_AFTER_MS = 24 * 60 * 60 * 1000

function seenTime(tab: OrderTab, recency: Record<string, TabRecency>): number {
  return recency[tab.id]?.seenAt ?? 0
}

function newestFirst<T extends OrderTab>(tabs: T[], time: (tab: T) => number): T[] {
  return tabs
    .map((tab, index) => ({ tab, index, at: time(tab) }))
    .sort((a, b) => b.at - a.at || a.index - b.index)
    .map((entry) => entry.tab)
}

// When a shell's agent last worked, from its last response or a turn started this run; 0 when
// unknown. Being on screen is not activity, so it never counts here.
export type ActivityOf = (tab: OrderTab) => number

// The order Reorganize gives a pane: shells by their latest agent activity, shells with none known
// after them by when they were last on screen, each followed by the other
// tabs of the task it works, then every other tab by when it was last on screen. `taskOf` names a
// shell's task and a document's task; when two shells work one task, its tabs follow the more
// recent shell. Ties keep the pane's current order, so a second Reorganize changes nothing.
export function reorganizedTabs<T extends OrderTab>(
  tabs: T[],
  recency: Record<string, TabRecency>,
  taskOf: (tab: T) => string | null = () => null,
  activityOf: ActivityOf = (tab) => recency[tab.id]?.activeAt ?? 0
): T[] {
  const allShells = tabs.filter((tab) => tab.kind === 'shell')
  const shells = [
    ...newestFirst(
      allShells.filter((tab) => activityOf(tab) > 0),
      (tab) => activityOf(tab)
    ),
    ...newestFirst(
      allShells.filter((tab) => activityOf(tab) === 0),
      (tab) => seenTime(tab, recency)
    )
  ]
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
