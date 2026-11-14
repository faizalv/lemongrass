// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { clearCommentDrafts, commentDraftsOf } from './conflictCommentStore'
import { commentMessages } from './conflictComments'
import { postToConflict } from './conflictThread'
import { applyFile } from './conflictVerdicts'
import type { FileDecisions } from './conflictDecisions'
import type { ProjectRef } from './workspace'

// The one Send. What the human queued, decisions on blocks and comments on lines, goes to the agent
// together so it wakes once: every post is quiet except the last, which is the one that wakes it.
// A comment with nothing else queued is the wake; with decisions too, the decisions go first and
// quiet. Returns the refusal, or null.
export async function sendQueued(
  project: ProjectRef,
  files: FileDecisions[]
): Promise<string | null> {
  const comments = commentDraftsOf(project.id).map((draft) => ({ ...draft }))
  const errors: string[] = []
  for (let i = 0; i < files.length; i++) {
    const last = i === files.length - 1
    const refused = await applyFile(project, files[i], comments.length > 0 || !last)
    if (refused)
      errors.push(`${files[i].path.slice(files[i].path.lastIndexOf('/') + 1)}: ${refused}`)
  }
  // A refused file stops the comments too, so nothing the human wrote goes out half sent.
  if (errors.length > 0) return errors.join(' ')
  if (comments.length === 0) return null
  const refused = await postToConflict(project, commentMessages(comments))
  if (refused) return refused
  clearCommentDrafts(project.id)
  return null
}
