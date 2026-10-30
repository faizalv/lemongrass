// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import type { GitConflictChunk, GitConflictFile } from '../../preload/types'

// Only the active project's workspace is mounted, so switching projects unmounts a resolver tab.
// Its work in progress is kept here by project and path, the one resolver tab per path, and handed
// back on the next mount.
export interface ConflictDraft {
  // The stages the draft was built from. A draft for other stages is stale.
  stages: string
  // The chunks the middle was loaded from, which Reset goes back to.
  chunks: GitConflictChunk[]
  finalNewline: boolean
  loadedText: string
  middle: unknown
  workingSeen: string | null
  focus: boolean
  sizes: number[]
  current: number | null
}

const drafts = new Map<string, ConflictDraft>()

function keyOf(projectId: string, path: string): string {
  return `${projectId}\n${path}`
}

export function stagesOf(file: Extract<GitConflictFile, { status: 'ok' }>): string {
  return JSON.stringify([file.base, file.left, file.right])
}

export function saveDraft(projectId: string, path: string, draft: ConflictDraft): void {
  drafts.set(keyOf(projectId, path), draft)
}

// Hands a draft back once, and only for the stages it was built from.
export function takeDraft(projectId: string, path: string, stages: string): ConflictDraft | null {
  const key = keyOf(projectId, path)
  const draft = drafts.get(key)
  drafts.delete(key)
  return draft && draft.stages === stages ? draft : null
}

export function dropDraft(projectId: string, path: string): void {
  drafts.delete(keyOf(projectId, path))
}
