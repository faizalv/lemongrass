// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { onBeforeUnmount, onMounted, reactive, watch } from 'vue'
import { conflictThreadOf } from './conflictThread'
import { modelHolds } from './conflictVerdicts'
import type { ProjectRef } from './workspace'

// Whether the tab holding a project's conflict is in the middle of a turn, so the screens can say
// the agent is working instead of leaving the human staring at a still page. One poll per project
// serves every screen that shows it, and runs only while one of them is mounted and an agent holds
// the conflict.

// Each poll runs lgrassd once, so this is as often as the screens can afford.
const POLL_MS = 2500

// null while it is not known, such as before the first poll or with no agent holding.
const busy = reactive<Record<string, boolean | null>>({})

interface Poll {
  users: number
  timer: ReturnType<typeof setInterval>
  stopWatch: () => void
}

const polls = new Map<string, Poll>()

export function agentBusyOf(projectId: string): boolean | null {
  return busy[projectId] ?? null
}

async function pollOnce(project: ProjectRef): Promise<void> {
  const assignee = conflictThreadOf(project.id).status?.assignee
  if (!assignee || !modelHolds(project.id)) {
    busy[project.id] = null
    return
  }
  try {
    const states = await window.api.tabSessions.states(project.path)
    busy[project.id] = states[assignee]?.busy ?? false
  } catch {
    busy[project.id] = null
  }
}

// Keeps the poll running while the calling component is mounted. A new message in the thread polls
// at once, so the spinner shows as soon as the agent picks the message up.
export function useAgentBusy(project: () => ProjectRef): void {
  let id: string | null = null
  onMounted(() => {
    const ref = project()
    id = ref.id
    const poll = polls.get(id)
    if (poll) {
      poll.users++
      return
    }
    void pollOnce(ref)
    polls.set(id, {
      users: 1,
      timer: setInterval(() => void pollOnce(project()), POLL_MS),
      stopWatch: watch(
        () => conflictThreadOf(ref.id).messages.length,
        () => void pollOnce(project())
      )
    })
  })
  onBeforeUnmount(() => {
    if (!id) return
    const poll = polls.get(id)
    if (!poll) return
    poll.users--
    if (poll.users > 0) return
    clearInterval(poll.timer)
    poll.stopWatch()
    polls.delete(id)
  })
}
