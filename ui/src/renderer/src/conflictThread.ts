// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { computed, onBeforeUnmount, onMounted, reactive, watch, type ComputedRef } from 'vue'
import { gitStateOf } from './git'
import { activeConflictKind } from './conflictScopes'
import { mergeMessages } from './conflictThreadModel'
import { saveConflictWork } from './conflictSave'
import { openConflictBeside, projectRefByPath, type ProjectRef } from './workspace'
import type {
  ConflictHumanMessage,
  ConflictReviewFile,
  ConflictThreadStatus,
  WorkgroupThreadMessage
} from '../../preload/types'

// The project's open conflict thread as the app last read it. Nothing about it lives only here:
// main finds it through `lgrass git conflict status`, and the messages are re-read from lgrass.
export interface ConflictThreadState {
  // undefined until the status was first read.
  status: ConflictThreadStatus | null | undefined
  messages: WorkgroupThreadMessage[]
  closed: boolean
  error: string | null
}

const PAGE = 200

const states = reactive<Record<string, ConflictThreadState>>({})
const statusRuns = new Map<string, Promise<void>>()
const readRuns = new Map<string, Promise<void>>()

export function conflictThreadOf(projectId: string): ConflictThreadState {
  return states[projectId] ?? { status: undefined, messages: [], closed: false, error: null }
}

function stateFor(projectId: string): ConflictThreadState {
  if (!states[projectId]) {
    states[projectId] = { status: undefined, messages: [], closed: false, error: null }
  }
  return states[projectId]
}

// Status runs lgrass and git, so it is read when the conflict itself changes, on a submit and after
// the human hands the conflict over, never on a timer.
export function refreshConflictStatus(project: ProjectRef): Promise<void> {
  const running = statusRuns.get(project.id)
  if (running) return running
  const run = (async () => {
    const state = stateFor(project.id)
    try {
      const status = await window.api.conflictAgent.status(project.path)
      if (status?.threadId !== state.status?.threadId) {
        state.messages = []
        state.closed = false
      }
      state.status = status
      state.error = null
    } catch (err) {
      state.error = err instanceof Error ? err.message : String(err)
    }
    await refreshConflictMessages(project)
  })().finally(() => statusRuns.delete(project.id))
  statusRuns.set(project.id, run)
  return run
}

// Model messages, notes and submits wake nobody, so the app learns of them only by reading the
// thread again. The first read pages back to the start, since notes on any file may be old.
export function refreshConflictMessages(project: ProjectRef): Promise<void> {
  const running = readRuns.get(project.id)
  if (running) return running
  const run = (async () => {
    const state = stateFor(project.id)
    const threadId = state.status?.threadId
    if (!threadId) return
    let before: number | undefined
    const first = state.messages.length === 0
    for (;;) {
      const result = await window.api.workgroup.thread(project.path, threadId, before, PAGE)
      if (state.status?.threadId !== threadId) return
      if (!result.ok || !result.thread) {
        state.error = result.error ?? 'Could not read the conflict thread.'
        return
      }
      state.error = null
      state.closed = result.thread.closed
      state.messages = mergeMessages(state.messages, result.thread.messages)
      if (!first || !result.thread.more || result.thread.messages.length === 0) return
      before = Math.min(...result.thread.messages.map((message) => message.id))
    }
  })().finally(() => readRuns.delete(project.id))
  readRuns.set(project.id, run)
  return run
}

// What makes the open thread possibly another one: the stopped operation and its refs.
function conflictKey(projectId: string): string {
  const status = gitStateOf(projectId).status
  if (!status) return ''
  return JSON.stringify([activeConflictKind(status), status.operationRefs ?? null])
}

// Keeps a project's conflict thread fresh while a component shows it: the status when the conflict
// changes, the messages on every git status poll.
export function useConflictThread(project: () => ProjectRef): ComputedRef<ConflictThreadState> {
  let stops: (() => void)[] = []
  onMounted(() => {
    void refreshConflictStatus(project())
    stops = [
      watch(
        () => conflictKey(project().id),
        (key, old) => {
          if (key !== old) void refreshConflictStatus(project())
        }
      ),
      watch(
        () => gitStateOf(project().id).revision,
        () => void refreshConflictMessages(project())
      ),
      watch(
        () => project().id,
        () => void refreshConflictStatus(project())
      )
    ]
  })
  onBeforeUnmount(() => stops.forEach((stop) => stop()))
  return computed(() => conflictThreadOf(project().id))
}

// The resolver's unsaved work goes to disk first, unstaged, so the model reads what the human sees,
// and its resolved ranges travel along so main can tell the model to keep them.
async function savedWork(project: ProjectRef): Promise<ConflictReviewFile[] | string> {
  const saved = await saveConflictWork(project)
  if (!saved.ok) return saved.error
  return saved.files.map((file) => ({
    path: file.path,
    resolved: file.resolved.map((r) => ({ ...r }))
  }))
}

// Hands the whole conflict to a tab: opens or reuses the conflict thread with that tab as its
// assignee and types the one-line start into it. Returns the refusal, or null.
export async function handConflictTo(project: ProjectRef, tabId: string): Promise<string | null> {
  try {
    const resolved = await savedWork(project)
    if (typeof resolved === 'string') return resolved
    const result = await window.api.conflictAgent.start(project.path, tabId, resolved)
    if (!result.ok) return result.reason
    await refreshConflictStatus(project)
    return null
  } catch (err) {
    return err instanceof Error ? err.message : String(err)
  }
}

// Posts the human's messages into the open conflict thread, which nudges the assignee. Returns the
// refusal, or null.
export async function postToConflict(
  project: ProjectRef,
  messages: ConflictHumanMessage[]
): Promise<string | null> {
  try {
    const resolved = await savedWork(project)
    if (typeof resolved === 'string') return resolved
    const plain = messages.map((message) => ({
      body: message.body,
      anchor: message.anchor ? { ...message.anchor } : null
    }))
    const result = await window.api.conflictAgent.post(project.path, plain, resolved)
    if (!result.ok) return result.reason
    await refreshConflictMessages(project)
    return null
  } catch (err) {
    return err instanceof Error ? err.message : String(err)
  }
}

// A submit opens the first submitted file in the model's pane. The thread is read first, so the
// resolver finds the submission when it mounts.
window.api.conflictAgent.onSubmitted(async (submitted) => {
  const project = projectRefByPath(submitted.projectPath)
  if (!project) return
  await refreshConflictStatus(project)
  const first = submitted.files[0]
  if (first) openConflictBeside(project, first, submitted.tabId)
})
