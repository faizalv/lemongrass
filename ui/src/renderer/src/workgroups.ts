import { reactive } from 'vue'
import type { WorkgroupInfo } from '../../preload/types'
import type { ProjectRef } from './workspace'

export interface WorkgroupState {
  groups: WorkgroupInfo[]
  folded: boolean
  confirming: number | null
  busy: boolean
  error: string | null
}

const states = reactive<Record<string, WorkgroupState>>({})
const refreshing = new Set<string>()

export function workgroupStateOf(projectId: string): WorkgroupState {
  if (!states[projectId]) {
    states[projectId] = { groups: [], folded: true, confirming: null, busy: false, error: null }
  }
  return states[projectId]
}

export async function refreshWorkgroups(project: ProjectRef): Promise<void> {
  if (refreshing.has(project.id)) return
  refreshing.add(project.id)
  try {
    const next = await window.api.workgroup.list(project.path)
    const state = workgroupStateOf(project.id)
    if (JSON.stringify(state.groups) !== JSON.stringify(next)) state.groups = next
    if (state.confirming !== null && !next.some((group) => group.id === state.confirming)) {
      state.confirming = null
    }
  } finally {
    refreshing.delete(project.id)
  }
}

export async function disbandWorkgroup(project: ProjectRef, groupId: number): Promise<void> {
  const state = workgroupStateOf(project.id)
  state.busy = true
  state.error = null
  try {
    const result = await window.api.workgroup.disband(project.path, groupId)
    if (!result.ok) state.error = result.error ?? 'Could not disband the workgroup.'
    state.confirming = null
    await refreshWorkgroups(project)
  } finally {
    state.busy = false
  }
}
