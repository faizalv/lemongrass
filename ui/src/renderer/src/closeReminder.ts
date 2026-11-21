// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { cacheLeftMs, cacheTtlKnown, closeReminderDue } from '../../preload/cacheState'
import { tabHookState } from './tabStates'
import { taskOfTab, taskPlansDone } from './taskOwners'
import { contextStore } from './usage'

export interface CloseReminder {
  tabId: string
  task: string
  // Whole minutes the cache has left, at least one.
  minutes: number
  // False when the lifetime is only assumed, which the wording must say.
  known: boolean
}

// A shell that finished every Plans item of its task, has sat idle long enough to have been left,
// and whose prompt cache is still warm: closing now reads the cached context instead of resending
// it. The pane banner and the notification watcher both read this, so they agree on when it is due.
export function closeReminderFor(
  projectPath: string,
  tabId: string,
  nowMs: number
): CloseReminder | null {
  if (tabHookState(tabId) !== 'idle') return null
  const task = taskOfTab(projectPath, tabId)
  const context = contextStore.tabs[tabId]
  if (!task || !context || !taskPlansDone(projectPath, task)) return null
  if (!closeReminderDue(context, nowMs)) return null
  return {
    tabId,
    task,
    minutes: Math.max(1, Math.floor(cacheLeftMs(context, nowMs) / 60_000)),
    known: cacheTtlKnown(context)
  }
}
