// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { Extension } from '@codemirror/state'
import { annotationGutter, type AnnotationRange, type CardReply } from './annotationGutter'
import {
  addCommentDraft,
  commentDraftsOf,
  editCommentDraft,
  removeCommentDraft
} from './conflictCommentStore'
import { sentComments } from './conflictComments'
import { conflictThreadOf } from './conflictThread'
import { agentVendorOf } from './reviewTargets'
import type { ProjectRef } from './workspace'
import type { ConflictCommentPane } from '../../preload/types'

// The comments of one pane that shows lines of a conflicted file, in Decisions or in Manual.
// It draws the human's saved comments, the ones already sent with what answered them, and takes new
// ones. What it does not know about the pane, the file line a document line is and the block a
// comment is about, the caller gives.
export interface CommentPane {
  project: ProjectRef
  path: string
  pane: ConflictCommentPane
  // A document line of the editor as the file line it is, and back; null when the pane cannot say.
  toFile: (docLine: number) => number | null
  toDoc: (fileLine: number) => number | null
  // The block a comment made at these file lines is about, 0 for the file.
  blockOf: (fromFile: number, toFile: number) => number
  // Whether a comment about this block shows in this pane.
  shows: (block: number) => boolean
}

export function commentGutter(spec: CommentPane): Extension {
  const { project } = spec

  function ranges(): AnnotationRange[] {
    const out: AnnotationRange[] = []
    for (const sent of sentComments(conflictThreadOf(project.id).messages)) {
      const { anchor } = sent
      if (anchor.path !== spec.path || anchor.pane !== spec.pane || !spec.shows(anchor.block))
        continue
      const fromLine = spec.toDoc(anchor.startLine)
      const toLine = spec.toDoc(anchor.endLine)
      if (fromLine === null || toLine === null) continue
      const replies: CardReply[] = sent.replies.map((reply) => ({
        author: reply.author,
        label: reply.label,
        body: reply.body,
        vendor: reply.author === 'tab' ? agentVendorOf(project.id, reply.tabId) : null
      }))
      out.push({ id: `m${sent.id}`, fromLine, toLine, comment: sent.body, sent: true, replies })
    }
    for (const draft of commentDraftsOf(project.id)) {
      if (draft.path !== spec.path || draft.pane !== spec.pane || !spec.shows(draft.block)) continue
      const fromLine = spec.toDoc(draft.startLine)
      const toLine = spec.toDoc(draft.endLine)
      if (fromLine === null || toLine === null) continue
      out.push({
        id: draft.id,
        fromLine,
        toLine,
        comment: draft.comment,
        reply: draft.replyTo !== undefined
      })
    }
    return out
  }

  return annotationGutter({
    ranges,
    targets: () => [],
    canAdd: (range) => spec.toFile(range.fromLine) !== null && spec.toFile(range.toLine) !== null,
    onAdd: (range, quote, comment) => {
      const startLine = spec.toFile(range.fromLine)
      const endLine = spec.toFile(range.toLine)
      if (startLine === null || endLine === null) return
      addCommentDraft(project.id, {
        path: spec.path,
        pane: spec.pane,
        block: spec.blockOf(startLine, endLine),
        startLine,
        endLine,
        quote,
        comment
      })
    },
    onEdit: (id, comment) => editCommentDraft(project.id, id, comment),
    onRemove: (id) => removeCommentDraft(project.id, id),
    // A reply is a draft of its own that points at the comment it answers, drawn under it.
    onReply: (id, comment) => {
      const parent = sentComments(conflictThreadOf(project.id).messages).find(
        (s) => `m${s.id}` === id
      )
      if (!parent) return
      const { anchor } = parent
      addCommentDraft(project.id, {
        path: anchor.path,
        pane: anchor.pane,
        block: anchor.block,
        startLine: anchor.startLine,
        endLine: anchor.endLine,
        quote: anchor.quote,
        comment,
        replyTo: parent.id
      })
    }
  })
}
