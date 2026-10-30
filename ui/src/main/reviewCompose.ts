// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import type {
  Annotation,
  AnnotationAnchor,
  ConflictAnchor,
  ConflictReviewScope,
  GitLineRange,
  ReviewScope,
  ReviewSubmission
} from '../preload/types'

// No value imports, so node --test loads this file straight from source.

// One conflicted file as it stands at submit. unresolved holds the marker blocks still on disk, or
// null when the file has no line markers to read (binary, or deleted on one side).
export interface ConflictFileState {
  path: string
  unresolved: GitLineRange[] | null
  resolved: GitLineRange[]
}

// What main read from disk for a scope, by the scope's kind.
export type ReviewDetails = { kind: 'conflict'; files: ConflictFileState[] }

// Each scope kind says what the review is about and what main found for it; each anchor kind says
// where one comment sits. The rest of the review never looks inside either.
interface ScopeFormatter<S extends ReviewScope> {
  heading(scope: S): string
  details(details: Extract<ReviewDetails, { kind: S['kind'] }>): string
}

interface AnchorFormatter<A extends AnnotationAnchor> {
  location(anchor: A): string
}

function rangeList(ranges: GitLineRange[]): string {
  const single = ranges.length === 1 && ranges[0].end <= ranges[0].start
  const spans = ranges.map((range) =>
    range.end <= range.start ? `${range.start}` : `${range.start}-${range.end}`
  )
  return `${single ? 'line' : 'lines'} ${spans.join(', ')}`
}

const conflictScopeFormatter: ScopeFormatter<ConflictReviewScope> = {
  heading(scope) {
    const subject =
      scope.operation === 'merge'
        ? `the merge of ${scope.rightRef} into ${scope.leftRef}`
        : scope.operation === 'rebase'
          ? `the rebase of ${scope.leftRef} onto ${scope.rightRef}`
          : `the stashed changes applied onto ${scope.rightRef}`
    return (
      `The human reviewed the conflicts in ${subject} and left this review for you. Line ` +
      'numbers are of each file as it is on disk now.'
    )
  },
  details({ files }) {
    if (files.length === 0) return ''
    const blocks = files.map((file) => {
      const lines = [file.path]
      if (file.unresolved === null) {
        lines.push(
          '  still in conflict: no line markers (binary, or deleted on one side), keep one side whole'
        )
      } else if (file.unresolved.length > 0) {
        lines.push(`  still in conflict: ${rangeList(file.unresolved)}`)
      } else {
        lines.push('  no conflict markers left, not staged yet')
      }
      if (file.resolved.length > 0) {
        lines.push(`  resolved by the human, keep as is: ${rangeList(file.resolved)}`)
      }
      return lines.join('\n')
    })
    return `Files:\n${blocks.join('\n')}`
  }
}

const conflictAnchorFormatter: AnchorFormatter<ConflictAnchor> = {
  location(anchor) {
    const pane =
      anchor.pane === 'middle'
        ? 'middle pane (the result)'
        : `${anchor.pane} pane (${anchor.pane === 'left' ? anchor.leftRef : anchor.rightRef})`
    const span = { start: anchor.startLine, end: anchor.endLine }
    return `${anchor.path}, ${rangeList([span])}, ${pane}`
  }
}

const scopeFormatters: {
  [K in ReviewScope['kind']]: ScopeFormatter<Extract<ReviewScope, { kind: K }>>
} = {
  conflict: conflictScopeFormatter
}

const anchorFormatters: {
  [K in AnnotationAnchor['kind']]: AnchorFormatter<Extract<AnnotationAnchor, { kind: K }>>
} = {
  conflict: conflictAnchorFormatter
}

function quoted(text: string): string {
  return text
    .replace(/\n+$/, '')
    .split('\n')
    .map((line) => `> ${line}`)
    .join('\n')
}

function item(index: number, annotation: Annotation): string {
  const formatter = anchorFormatters[annotation.anchor.kind] as AnchorFormatter<AnnotationAnchor>
  const parts = [`${index + 1}. ${formatter.location(annotation.anchor)}:`]
  if (annotation.quote.trim()) parts.push(quoted(annotation.quote))
  parts.push(annotation.comment.trim())
  return parts.join('\n')
}

// Lemongrass composes this text, so it starts with [lg] like every line lemongrass types, and its
// heading says the review is the human's. Then come the overall comment, what main found for the
// scope, and each comment with where it sits and the text it is on. Empty when the human wrote
// neither an overall comment nor a block comment.
export function composeReview(submission: ReviewSubmission, details: ReviewDetails): string {
  const annotations = submission.annotations.filter((annotation) => annotation.comment.trim())
  const message = submission.message.trim()
  if (annotations.length === 0 && !message) return ''
  const formatter = scopeFormatters[submission.scope.kind]
  const body = [
    formatter.heading(submission.scope),
    message && `Overall comment: ${message}`,
    details.kind === submission.scope.kind ? formatter.details(details) : '',
    annotations.length > 0 &&
      `Comments:\n${annotations.map((annotation, index) => item(index, annotation)).join('\n\n')}`
  ]
  return `[lg] ${body.filter(Boolean).join('\n\n')}`
}
