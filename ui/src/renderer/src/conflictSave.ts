// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { workSourcesOf, type ConflictDraft, type ConflictWork } from './conflictDrafts'
import { isCrlf, savedText, stripChunks, type MiddleChunk } from './conflictModel'
import type { ProjectRef } from './workspace'
import type { GitLineRange } from '../../preload/types'

export type ConflictSaveResult =
  { ok: true; files: { path: string; resolved: GitLineRange[] }[] } | { ok: false; error: string }

function draftWork(path: string, draft: ConflictDraft): ConflictWork | null {
  const middle = draft.middle as { doc: string; chunks: MiddleChunk[] }
  if (middle.doc === draft.loadedText) return null
  const saved = savedText(
    middle.doc,
    middle.chunks,
    stripChunks(draft.chunks),
    draft.refs,
    draft.finalNewline,
    isCrlf(draft.chunks)
  )
  return { path, ...saved, markSeen: (text) => (draft.workingSeen = text) }
}

// Every conflicted file of the project with work in a resolver tab, mounted or kept as a draft.
function conflictWorkOf(projectId: string): ConflictWork[] {
  const { live, drafts } = workSourcesOf(projectId)
  return [...live, ...drafts.flatMap(({ path, draft }) => draftWork(path, draft) ?? [])]
}

// Writes the human's partial work in every resolver tab of the project to disk, without staging,
// so a review can point the model at it. Stops at the first failed write, so no review goes out
// describing a file that was not saved.
export async function saveConflictWork(project: ProjectRef): Promise<ConflictSaveResult> {
  const files: { path: string; resolved: GitLineRange[] }[] = []
  for (const work of conflictWorkOf(project.id)) {
    const saved = await window.api.git.conflictSave(project.path, work.path, work.text)
    if (!saved.ok) return { ok: false, error: `${work.path}: ${saved.error}` }
    work.markSeen(work.text)
    files.push({ path: work.path, resolved: work.resolved })
  }
  return { ok: true, files }
}
