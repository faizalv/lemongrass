// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import { planStatus, type PlanStatus } from './planMarks'

// Per project path, each shell tab's task: the scratchpad task its session most recently edited a
// file in, derived by lgrassd from the edit log. Edits land during a turn, so the map is refreshed
// when a turn ends and before Reorganize reads it, together with each owned task's plan status.
const owners = reactive<Record<string, Record<string, string>>>({})
const statuses = reactive<Record<string, Record<string, PlanStatus>>>({})

export function taskOfTab(projectPath: string, tabId: string): string | null {
  return owners[projectPath]?.[tabId] ?? null
}

// A task without a readable prd.md counts as open.
export function taskPlanStatus(projectPath: string, slug: string): PlanStatus {
  return statuses[projectPath]?.[slug] ?? 'open'
}

export async function refreshTaskOwners(projectPath: string): Promise<void> {
  const next = await window.api.tabSessions.owners(projectPath).catch(() => ({}))
  owners[projectPath] = next
  const status: Record<string, PlanStatus> = {}
  await Promise.all(
    [...new Set(Object.values(next))].map(async (slug) => {
      const prd = await window.api.biblio
        .read(projectPath, `scratchpad/${slug}/prd.md`)
        .catch(() => null)
      status[slug] = prd === null ? 'open' : planStatus(prd)
    })
  )
  statuses[projectPath] = status
}

// The task a biblio document path belongs to, `scratchpad/<slug>/...`, never the archive.
export function taskOfDocPath(path: string): string | null {
  const parts = path.split('/')
  if (parts.length < 3 || parts[0] !== 'scratchpad' || parts[1] === 'archive') return null
  return parts[1]
}
