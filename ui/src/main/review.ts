// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { ipcMain } from 'electron'
import { conflictBlockRanges } from './gitConflict'
import { readEnv, repoRoot, run } from './gitRun'
import { composeReview, type ReviewDetails } from './reviewCompose'
import { pastePrompt, type PasteOutcome } from './tabNudge'
import { tabStates } from './tabSessions'
import type {
  Annotation,
  ConflictReviewScope,
  GitLineRange,
  ReviewSubmission,
  ReviewSubmitResult
} from '../preload/types'

const OUTCOME_REASONS: Record<Exclude<PasteOutcome, 'typed'>, string> = {
  closed: 'The tab is closed',
  typing: 'You are typing in that tab',
  empty: 'Nothing to send'
}

function isAnnotation(value: unknown): value is Annotation {
  const annotation = value as Annotation
  return (
    typeof annotation?.id === 'string' &&
    typeof annotation.quote === 'string' &&
    typeof annotation.comment === 'string' &&
    annotation.anchor?.kind === 'conflict' &&
    typeof annotation.anchor.path === 'string' &&
    Number.isInteger(annotation.anchor.startLine) &&
    Number.isInteger(annotation.anchor.endLine)
  )
}

function isRange(value: unknown): value is GitLineRange {
  const range = value as GitLineRange
  return Number.isInteger(range?.start) && Number.isInteger(range.end)
}

function isConflictScope(value: unknown): value is ConflictReviewScope {
  const scope = value as ConflictReviewScope
  return (
    scope?.kind === 'conflict' &&
    ['merge', 'rebase', 'stash'].includes(scope.operation) &&
    typeof scope.leftRef === 'string' &&
    typeof scope.rightRef === 'string' &&
    Array.isArray(scope.files) &&
    scope.files.every(
      (file) =>
        typeof file?.path === 'string' &&
        Array.isArray(file.resolved) &&
        file.resolved.every(isRange)
    )
  )
}

// Every file still unmerged in the index, with the blocks its markers still hold on disk and the
// ranges the human resolved there. A file the renderer sent that was staged since is left out.
async function conflictDetails(
  projectPath: string,
  scope: ConflictReviewScope
): Promise<ReviewDetails | string> {
  const root = await repoRoot(projectPath)
  if (!root) return 'This project is not a git repository'
  const listed = await run(root, ['diff', '--name-only', '-z', '--diff-filter=U'], { env: readEnv })
  if (listed.code !== 0) return 'Could not list the conflicted files'
  const resolved = new Map(scope.files.map((file) => [file.path, file.resolved]))
  const files = [...new Set(listed.stdout.split('\0').filter(Boolean))].map((path) => ({
    path,
    unresolved: conflictBlockRanges(root, path),
    resolved: resolved.get(path) ?? []
  }))
  return { kind: 'conflict', files }
}

// The text is lemongrass's, marked [lg], carrying the human's review. The tab state is checked
// again here because the picker's view of it can be a poll old.
async function submit(
  projectPath: string,
  tabId: string,
  submission: ReviewSubmission
): Promise<ReviewSubmitResult> {
  if (
    typeof submission?.message !== 'string' ||
    !Array.isArray(submission.annotations) ||
    !submission.annotations.every(isAnnotation) ||
    !isConflictScope(submission.scope)
  ) {
    return { ok: false, reason: 'The review is not valid' }
  }
  const details = await conflictDetails(projectPath, submission.scope)
  if (typeof details === 'string') return { ok: false, reason: details }
  const text = composeReview(submission, details)
  if (!text) return { ok: false, reason: OUTCOME_REASONS.empty }
  const state = (await tabStates(projectPath))[tabId]
  if (state?.busy) return { ok: false, reason: state.reason ?? 'The tab is busy' }
  const outcome = await pastePrompt(tabId, text)
  return outcome === 'typed' ? { ok: true } : { ok: false, reason: OUTCOME_REASONS[outcome] }
}

export function registerReviewHandlers(): void {
  ipcMain.handle(
    'review:submit',
    (_event, projectPath: string, tabId: string, submission: ReviewSubmission) =>
      submit(projectPath, tabId, submission)
  )
}
