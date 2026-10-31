// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import type {
  Annotation,
  AnnotationAnchor,
  ConflictAnchor,
  ConflictReviewScope,
  GitConflictOperation,
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

interface ConflictSides {
  operation: GitConflictOperation
  leftRef: string
  rightRef: string
}

// Sides follow the resolver's panes, so a merge brings the right into the left and a rebase
// replays the left onto the right.
function conflictSubject({ operation, leftRef, rightRef }: ConflictSides): string {
  return operation === 'merge'
    ? `the merge of ${rightRef} into ${leftRef}`
    : operation === 'rebase'
      ? `the rebase of ${leftRef} onto ${rightRef}`
      : `the stashed changes applied onto ${rightRef}`
}

const conflictScopeFormatter: ScopeFormatter<ConflictReviewScope> = {
  heading(scope) {
    return (
      `The human reviewed the conflicts in ${conflictSubject(scope)} and left this review for ` +
      'you. Line numbers are of each file as it is on disk now.'
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

// The line typed into the tab the human hands a conflict to. It names only refs, which pass git's
// ref format check and so hold no spaces or control characters; everything else the model pulls
// itself through `lgrass git conflict show`.
export function composeStartLine(sides: ConflictSides): string {
  return `[lg] the human handed you the conflicts in ${conflictSubject(sides)}. Run lgrass how git resolve.`
}

export interface CommitIntent {
  hash: string
  subject: string
  body: string
}

// The commits one side brings that touched the conflicted files, newest first. more counts the
// ones left out, and missing says why git could not tell.
export interface SideIntent {
  commits: CommitIntent[]
  more: number
  missing: string | null
}

export interface ConflictIntent {
  left: SideIntent
  right: SideIntent
}

export interface ConflictShow extends ConflictSides {
  threadId: string
  files: ConflictFileState[]
  intent: ConflictIntent
}

function sideTitles(sides: ConflictSides): { left: string; right: string } {
  const { operation, leftRef, rightRef } = sides
  if (operation === 'merge') {
    return {
      left: `${leftRef}, the current branch, commits that touched these files since the merge base`,
      right: `${rightRef}, the branch being merged, commits that touched these files since the merge base`
    }
  }
  if (operation === 'rebase') {
    return {
      left: `${leftRef}, the branch being rebased, the commit being replayed now`,
      right:
        `${rightRef}, the upstream, commits that touched these files since ${leftRef} branched ` +
        `off, the commits of ${leftRef} already replayed included`
    }
  }
  return {
    left: 'the stashed changes, the newest stash, which is most likely the one applied',
    right: `${rightRef}, the current branch, commits that touched these files since the stash was made`
  }
}

function sideSection(title: string, side: SideIntent): string {
  const lines = [`Intent of ${title}:`]
  if (side.missing) lines.push(`  unknown, ${side.missing}`)
  else if (side.commits.length === 0) lines.push('  none')
  for (const commit of side.commits) {
    lines.push(`- ${commit.hash} ${commit.subject}`)
    if (commit.body) lines.push(...commit.body.split('\n').map((line) => `    ${line}`.trimEnd()))
  }
  if (side.more > 0) lines.push(`  and ${side.more} more`)
  return lines.join('\n')
}

// What `lgrass git conflict show` prints for the tab the human handed the conflict to: the same
// files section a review carries, the conflict thread, and what each side meant.
export function composeShow(show: ConflictShow): string {
  const titles = sideTitles(show)
  const body = [
    `The human handed you the conflicts in ${conflictSubject(show)}. Line numbers are of each ` +
      'file as it is on disk now.',
    `Conflict thread: ${show.threadId}`,
    conflictScopeFormatter.details({ kind: 'conflict', files: show.files }),
    sideSection(titles.left, show.intent.left),
    sideSection(titles.right, show.intent.right)
  ]
  return body.filter(Boolean).join('\n\n')
}
