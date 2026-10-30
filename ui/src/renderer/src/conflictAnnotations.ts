// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { watch } from 'vue'
import type { Extension } from '@codemirror/state'
import {
  addAnnotation,
  clearReview,
  editAnnotation,
  projectsWithReviews,
  removeAnnotation,
  reviewOf,
  scopesOf
} from './annotations'
import { annotationGutter, refreshAnnotations, type LineRange } from './annotationGutter'
import { activeConflictKind, conflictScope, endedConflictScopes } from './conflictScopes'
import { gitStateOf } from './git'
import type { ProjectRef } from './workspace'
import type { ConflictPane, GitConflictOperation } from '../../preload/types'

export { conflictScope } from './conflictScopes'
export const refreshConflictComments = refreshAnnotations

// One pane of the resolver. Pane lines are 1-based and inclusive in the pane's current document,
// disk lines in the working file as it is on disk; the resolver owns the mapping between them.
export interface ConflictPaneContext {
  project: ProjectRef
  path: string
  pane: ConflictPane
  operation: GitConflictOperation
  leftRef: string
  rightRef: string
  toDisk: (fromLine: number, toLine: number) => { startLine: number; endLine: number } | null
  fromDisk: (startLine: number, endLine: number) => LineRange | null
  chunks: () => LineRange[]
}

export function conflictCommentExtension(ctx: ConflictPaneContext): Extension {
  const scope = conflictScope(ctx.operation, ctx.leftRef, ctx.rightRef)
  return annotationGutter({
    // A comment whose disk lines no longer map into this pane stays in the review, unmarked.
    ranges: () =>
      reviewOf(ctx.project.id, scope).annotations.flatMap((annotation) => {
        const anchor = annotation.anchor
        if (anchor.kind !== 'conflict' || anchor.path !== ctx.path || anchor.pane !== ctx.pane) {
          return []
        }
        const lines = ctx.fromDisk(anchor.startLine, anchor.endLine)
        return lines ? [{ id: annotation.id, comment: annotation.comment, ...lines }] : []
      }),
    targets: ctx.chunks,
    canAdd: (range) => ctx.toDisk(range.fromLine, range.toLine) !== null,
    onAdd: (range, quote, comment) => {
      const disk = ctx.toDisk(range.fromLine, range.toLine)
      if (!disk) return
      const anchor = {
        kind: 'conflict' as const,
        path: ctx.path,
        startLine: disk.startLine,
        endLine: disk.endLine,
        pane: ctx.pane,
        operation: ctx.operation,
        leftRef: ctx.leftRef,
        rightRef: ctx.rightRef
      }
      addAnnotation(ctx.project.id, scope, anchor, quote, comment)
    },
    onEdit: (id, comment) => editAnnotation(ctx.project.id, scope, id, comment),
    onRemove: (id) => removeAnnotation(ctx.project.id, scope, id),
    side: ctx.pane === 'right' ? 'after' : 'before'
  })
}

// A conflict review ends with its merge, rebase or stash reapply, whichever screen is open.
watch(
  () =>
    projectsWithReviews().map((projectId) => ({
      projectId,
      kind: activeConflictKind(gitStateOf(projectId).status)
    })),
  (projects) => {
    for (const { projectId, kind } of projects) {
      if (kind === undefined) continue
      for (const scope of endedConflictScopes(scopesOf(projectId), kind)) {
        clearReview(projectId, scope)
      }
    }
  }
)
