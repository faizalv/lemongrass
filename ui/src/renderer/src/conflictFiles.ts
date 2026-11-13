// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import {
  computed,
  effectScope,
  onBeforeUnmount,
  onMounted,
  reactive,
  watch,
  type ComputedRef
} from 'vue'
import { gitStateOf } from './git'
import { conflictThreadOf } from './conflictThread'
import { buildProposal } from './conflictModel'
import { fileDecisions, type FileDecisions } from './conflictDecisions'
import type { ProjectRef } from './workspace'
import type { GitConflictFile } from '../../preload/types'

// Every conflicted file of a project as main reads it, while something on screen needs them all:
// Decisions, the file picker's hints and the git panel's file states read the same copy. Loaded
// again on every git status refresh, since a model writing a file shows up there, and only while
// a model holds the conflict, the one time anything reads more than the file shown.

type OkFile = Extract<GitConflictFile, { status: 'ok' }>

const files = reactive<Record<string, Record<string, OkFile>>>({})
const users = new Map<string, number>()
const loading = new Set<string>()
const again = new Set<string>()

function conflictedPaths(projectId: string): string[] {
  return (gitStateOf(projectId).status?.files ?? [])
    .filter((file) => file.group === 'conflicted')
    .map((file) => file.path)
}

async function load(project: ProjectRef): Promise<void> {
  if (loading.has(project.id)) {
    again.add(project.id)
    return
  }
  loading.add(project.id)
  try {
    do {
      again.delete(project.id)
      const paths = conflictedPaths(project.id)
      const next: Record<string, OkFile> = {}
      for (const path of paths) {
        try {
          const file = await window.api.git.conflict(project.path, path)
          if (file.status === 'ok') next[path] = file
        } catch {
          // A file that cannot be read is left out until the next refresh.
        }
      }
      const held = files[project.id] ?? {}
      // Replaced only where a file changed, so views keyed on a file do not redraw for nothing.
      const merged: Record<string, OkFile> = {}
      for (const [path, file] of Object.entries(next)) {
        merged[path] = JSON.stringify(held[path]) === JSON.stringify(file) ? held[path] : file
      }
      files[project.id] = merged
    } while (again.has(project.id))
  } finally {
    loading.delete(project.id)
  }
}

// One watcher per project however many views read its files, stopped with the last of them.
const watchers = new Map<string, () => void>()

// Keeps a project's conflicted files fresh while the caller is mounted.
export function useConflictFiles(project: () => ProjectRef): ComputedRef<Record<string, OkFile>> {
  let id = ''
  onMounted(() => {
    id = project().id
    users.set(id, (users.get(id) ?? 0) + 1)
    if (watchers.has(id)) return
    const scope = effectScope(true)
    const ref = project()
    scope.run(() =>
      watch(
        () => [gitStateOf(ref.id).revision, !!conflictThreadOf(ref.id).status],
        ([, held]) => {
          if (held) void load(ref)
        },
        { immediate: true }
      )
    )
    watchers.set(id, () => scope.stop())
  })
  onBeforeUnmount(() => {
    const left = (users.get(id) ?? 1) - 1
    if (left > 0) {
      users.set(id, left)
      return
    }
    users.delete(id)
    watchers.get(id)?.()
    watchers.delete(id)
    delete files[id]
  })
  return computed(() => files[project().id] ?? {})
}

// Every conflicted file's blocks and where they stand, in git status order.
export function useFileDecisions(project: () => ProjectRef): ComputedRef<FileDecisions[]> {
  const loaded = useConflictFiles(project)
  return computed(() => {
    const messages = conflictThreadOf(project().id).messages
    return conflictedPaths(project().id).flatMap((path) => {
      const file = loaded.value[path]
      if (!file) return []
      const mapped =
        file.chunks && file.working !== null
          ? buildProposal(file.chunks, file.working, file.workingChunks)
          : null
      return [fileDecisions(file, mapped, messages)]
    })
  })
}
