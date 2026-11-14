// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { computed, onBeforeUnmount, onMounted, reactive, watch, type ComputedRef } from 'vue'
import { gitStateOf } from './git'
import { activeConflictKind } from './conflictScopes'
import { mergeMessages } from './conflictThreadModel'
import { saveConflictWork } from './conflictSave'
import { openConflictBeside, projectRefByPath, type ProjectRef } from './workspace'
import type {
  ConflictHumanMessage,
  ConflictThreadStatus,
  ConflictVerdictBatch,
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

// The model's decisions, questions and submits wake nobody, so the app learns of them only by
// reading the thread again. The first read pages back to the start, since a block's latest word
// may be old.
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

// Hands the whole conflict to a tab: the resolver's unsaved work goes to disk first, unstaged, so
// the model reads what the human sees; the thread opens or is reused with that tab as its
// assignee, the blocks already resolved go along as settled, read by settled once the work is
// saved, and the one-line start is typed into the tab. asked is the blocks the human asks about,
// and messages the comments and words sent with it, both posted with the hand-over so the model
// wakes once and finds them. Returns the refusal, or null.
export async function handConflictTo(
  project: ProjectRef,
  tabId: string,
  settled: () => Promise<ConflictVerdictBatch[]>,
  asked: ConflictVerdictBatch | null = null,
  messages: ConflictHumanMessage[] | null = null
): Promise<string | null> {
  try {
    const saved = await saveConflictWork(project)
    if (!saved.ok) return saved.error
    const batches = (await settled()).map((batch) => ({
      path: batch.path,
      verdicts: batch.verdicts.map((v) => ({ ...v }))
    }))
    const question = asked
      ? { path: asked.path, verdicts: asked.verdicts.map((v) => ({ ...v })) }
      : null
    const sent = messages ? plainMessages(messages) : null
    const result = await window.api.conflictAgent.start(
      project.path,
      tabId,
      batches,
      question,
      sent
    )
    if (!result.ok) return result.reason
    await refreshConflictStatus(project)
    return null
  } catch (err) {
    return err instanceof Error ? err.message : String(err)
  }
}

// Messages as plain data, since a reactive proxy cannot cross IPC.
function plainMessages(messages: ConflictHumanMessage[]): ConflictHumanMessage[] {
  return messages.map((message) => ({
    body: message.body,
    anchor: message.anchor ? { ...message.anchor } : null,
    ...(message.replyTo ? { replyTo: message.replyTo } : {})
  }))
}

// Posts the human's plain messages into the open conflict thread, which nudges the assignee.
// Returns the refusal, or null.
export async function postToConflict(
  project: ProjectRef,
  messages: ConflictHumanMessage[]
): Promise<string | null> {
  try {
    const result = await window.api.conflictAgent.post(project.path, plainMessages(messages))
    if (!result.ok) return result.reason
    await refreshConflictMessages(project)
    return null
  } catch (err) {
    return err instanceof Error ? err.message : String(err)
  }
}

// The end of a model's round brings the conflict tab forward beside the model's tab without taking
// the human's focus. The thread is read first, so Decisions shows the round when it mounts.
window.api.conflictAgent.onSubmitted(async (submitted) => {
  const project = projectRefByPath(submitted.projectPath)
  if (!project) return
  await refreshConflictStatus(project)
  const first = submitted.files[0]
  if (first) openConflictBeside(project, first, submitted.tabId)
})
