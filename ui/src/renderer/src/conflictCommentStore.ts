// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import type { CommentDraft } from './conflictComments'

// The comments the human wrote on lines and has not sent, per project and in memory only. They go
// out together with the decisions on the one Send, so a draft is only ever a card here until then.

const drafts = reactive<Record<string, CommentDraft[]>>({})
let counter = 0

const NONE: CommentDraft[] = []

// For reads, which a render or a computed may run: never creates state.
export function commentDraftsOf(projectId: string): CommentDraft[] {
  return drafts[projectId] ?? NONE
}

export function addCommentDraft(projectId: string, draft: Omit<CommentDraft, 'id'>): string {
  const id = `c${++counter}`
  const list = (drafts[projectId] ??= [])
  list.push({ ...draft, id })
  return id
}

export function editCommentDraft(projectId: string, id: string, comment: string): void {
  const draft = drafts[projectId]?.find((d) => d.id === id)
  if (draft) draft.comment = comment
}

export function removeCommentDraft(projectId: string, id: string): void {
  const list = drafts[projectId]
  if (!list) return
  const at = list.findIndex((d) => d.id === id)
  if (at !== -1) list.splice(at, 1)
}

export function clearCommentDrafts(projectId: string): void {
  delete drafts[projectId]
}
