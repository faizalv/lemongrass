// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import {
  IDLE_TASK_REMINDER_MS,
  cacheLeftMs,
  cacheTtlKnown,
  closeReminderDue,
  closeReminderDueAt
} from '../../preload/cacheState'
import { tabHookState } from './tabStates'
import { taskOfTab, taskPlanStatus } from './taskOwners'
import { contextStore } from './usage'

// The pane banners and the notification watcher both read these, so they agree on when a reminder
// is due. Each reminder belongs to one idle stretch, named by the tab's last response (seenAt): a
// dismiss or a notification holds for that stretch, and the next one reminds again.

export interface CloseReminder {
  tabId: string
  task: string
  seenAt: number
  // When it became due, in epoch milliseconds.
  dueAt: number
  // Whole minutes the cache has left, at least one.
  minutes: number
  // False when the lifetime is only assumed, which the wording must say.
  known: boolean
}

// A shell that finished every Plans item of its task, has sat idle long enough to have been left,
// and whose prompt cache is still warm: closing now reads the cached context instead of resending
// it.
export function closeReminderFor(
  projectPath: string,
  tabId: string,
  nowMs: number
): CloseReminder | null {
  if (tabHookState(tabId) !== 'idle') return null
  const task = taskOfTab(projectPath, tabId)
  const context = contextStore.tabs[tabId]
  if (!task || !context || taskPlanStatus(projectPath, task) !== 'done') return null
  if (!closeReminderDue(context, nowMs)) return null
  return {
    tabId,
    task,
    seenAt: context.seen_at,
    dueAt: closeReminderDueAt(context),
    minutes: Math.max(1, Math.floor(cacheLeftMs(context, nowMs) / 60_000)),
    known: cacheTtlKnown(context)
  }
}

export interface IdleTaskReminder {
  tabId: string
  task: string
  seenAt: number
  dueAt: number
  // Time since the tab's last response.
  idleMs: number
  // Minutes its cache stays warm, only when the lifetime is known and has not run out.
  warmMinutes: number | null
  // Only the user approval item is open: the work waits for the human, not the agent.
  awaitingApproval: boolean
}

// A shell that owns a task with open Plans items and has sat idle long enough that the human may
// have forgotten it.
export function idleTaskReminderFor(
  projectPath: string,
  tabId: string,
  nowMs: number
): IdleTaskReminder | null {
  if (tabHookState(tabId) !== 'idle') return null
  const task = taskOfTab(projectPath, tabId)
  const context = contextStore.tabs[tabId]
  // Codex context can be sampled while a turn is still running.
  if (!task || !context || context.vendor === 'codex') return null
  const status = taskPlanStatus(projectPath, task)
  if (status === 'done') return null
  const dueAt = context.seen_at * 1000 + IDLE_TASK_REMINDER_MS
  if (nowMs < dueAt) return null
  const left = cacheLeftMs(context, nowMs)
  return {
    tabId,
    task,
    seenAt: context.seen_at,
    dueAt,
    idleMs: nowMs - context.seen_at * 1000,
    warmMinutes: cacheTtlKnown(context) && left > 0 ? Math.max(1, Math.floor(left / 60_000)) : null,
    awaitingApproval: status === 'approval'
  }
}
