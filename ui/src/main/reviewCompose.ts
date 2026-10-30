// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import type {
  Annotation,
  AnnotationAnchor,
  ConflictAnchor,
  ReviewSubmission
} from '../preload/types'

// No value imports, so node --test loads this file straight from source.

// Each anchor kind says what its review is about and where one of its comments sits. The rest of
// the review never looks inside an anchor.
interface AnchorFormatter<A extends AnnotationAnchor> {
  heading(anchor: A): string
  location(anchor: A): string
}

function lineSpan(start: number, end: number): string {
  return end <= start ? `line ${start}` : `lines ${start} to ${end}`
}

const conflictFormatter: AnchorFormatter<ConflictAnchor> = {
  heading(anchor) {
    const subject =
      anchor.operation === 'merge'
        ? `the merge of ${anchor.rightRef} into ${anchor.leftRef}`
        : anchor.operation === 'rebase'
          ? `the rebase of ${anchor.leftRef} onto ${anchor.rightRef}`
          : `the stashed changes applied onto ${anchor.rightRef}`
    return (
      `The human reviewed the conflicts in ${subject} and left these comments for you. Line ` +
      'numbers are of each file as it is on disk, conflict markers included while it is unresolved.'
    )
  },
  location(anchor) {
    const pane =
      anchor.pane === 'middle'
        ? 'middle pane (the result)'
        : `${anchor.pane} pane (${anchor.pane === 'left' ? anchor.leftRef : anchor.rightRef})`
    return `${anchor.path}, ${lineSpan(anchor.startLine, anchor.endLine)}, ${pane}`
  }
}

const formatters: {
  [K in AnnotationAnchor['kind']]: AnchorFormatter<Extract<AnnotationAnchor, { kind: K }>>
} = {
  conflict: conflictFormatter
}

function formatterOf(anchor: AnnotationAnchor): AnchorFormatter<AnnotationAnchor> {
  return formatters[anchor.kind] as AnchorFormatter<AnnotationAnchor>
}

function quoted(text: string): string {
  return text
    .replace(/\n+$/, '')
    .split('\n')
    .map((line) => `> ${line}`)
    .join('\n')
}

function item(index: number, annotation: Annotation): string {
  const parts = [`${index + 1}. ${formatterOf(annotation.anchor).location(annotation.anchor)}:`]
  if (annotation.quote.trim()) parts.push(quoted(annotation.quote))
  parts.push(annotation.comment.trim())
  return parts.join('\n')
}

// Lemongrass composes this text, so it starts with [lg] like every line lemongrass types, and its
// heading says the comments are the human's. Then come the overall message and each comment with
// where it sits and the text it is on. Empty when there is nothing to send.
export function composeReview(submission: ReviewSubmission): string {
  const annotations = submission.annotations.filter((annotation) => annotation.comment.trim())
  const message = submission.message.trim()
  if (annotations.length === 0 && !message) return ''
  const headings = [
    ...new Set(
      annotations.map((annotation) => formatterOf(annotation.anchor).heading(annotation.anchor))
    )
  ]
  if (headings.length === 0) headings.push('The human sent you this review.')
  const body = [
    ...headings,
    message,
    ...annotations.map((annotation, index) => item(index, annotation))
  ]
  return `[lg] ${body.filter(Boolean).join('\n\n')}`
}
