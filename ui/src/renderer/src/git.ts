import { reactive } from 'vue'
import type { GitFile, GitStatus } from '../../preload/types'
import type { ProjectRef } from './workspace'

export interface GitState {
  status: GitStatus | null
  revision: number
  folded: boolean
  message: string
  unchecked: string[]
  busy: 'commit' | 'push' | null
  error: string | null
}

const states = reactive<Record<string, GitState>>({})
const refreshing = new Set<string>()

export function gitStateOf(projectId: string): GitState {
  if (!states[projectId]) {
    states[projectId] = {
      status: null,
      revision: 0,
      folded: true,
      message: '',
      unchecked: [],
      busy: null,
      error: null
    }
  }
  return states[projectId]
}

export async function refreshGit(project: ProjectRef): Promise<void> {
  if (refreshing.has(project.id)) return
  refreshing.add(project.id)
  try {
    const next = await window.api.git.status(project.path)
    const state = gitStateOf(project.id)
    if (JSON.stringify(state.status) !== JSON.stringify(next)) state.status = next
    // A file that left the change list starts checked again if it changes later.
    const present = new Set(next.files.map((file) => file.path))
    if (state.unchecked.some((path) => !present.has(path))) {
      state.unchecked = state.unchecked.filter((path) => present.has(path))
    }
    state.revision += 1
  } finally {
    refreshing.delete(project.id)
  }
}

export function isChecked(projectId: string, path: string): boolean {
  return !gitStateOf(projectId).unchecked.includes(path)
}

export function toggleFile(projectId: string, path: string): void {
  const state = gitStateOf(projectId)
  state.unchecked = isChecked(projectId, path)
    ? [...state.unchecked, path]
    : state.unchecked.filter((entry) => entry !== path)
}

export function setAllChecked(projectId: string, files: GitFile[], checked: boolean): void {
  gitStateOf(projectId).unchecked = checked ? [] : files.map((file) => file.path)
}

export async function commitChanges(project: ProjectRef, andPush: boolean): Promise<void> {
  const state = gitStateOf(project.id)
  if (state.busy) return
  state.error = null
  state.busy = 'commit'
  const files = state.status?.files ?? []
  const selected = files.filter((file) => !state.unchecked.includes(file.path))
  // A rename must carry its old path too, or the deletion half stays behind.
  const paths =
    selected.length < files.length
      ? selected.flatMap((file) => (file.origPath ? [file.path, file.origPath] : [file.path]))
      : undefined
  const committed = await window.api.git.commit(project.path, state.message, paths)
  if (!committed.ok) {
    state.error = committed.error
    state.busy = null
    await refreshGit(project)
    return
  }
  state.message = ''
  if (andPush) {
    state.busy = 'push'
    const pushed = await window.api.git.push(project.path)
    if (!pushed.ok) state.error = `The commit was created, but the push failed. ${pushed.error}`
  }
  state.busy = null
  await refreshGit(project)
}
