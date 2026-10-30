// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { ipcMain } from 'electron'
import { composeReview } from './reviewCompose'
import { pastePrompt, type PasteOutcome } from './tabNudge'
import { tabStates } from './tabSessions'
import type { Annotation, ReviewSubmission, ReviewSubmitResult } from '../preload/types'

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

// A review is the human's own prompt, so it carries no [lg] prefix. The tab state is checked again
// here because the picker's view of it can be a poll old.
async function submit(
  projectPath: string,
  tabId: string,
  submission: ReviewSubmission
): Promise<ReviewSubmitResult> {
  if (
    typeof submission?.message !== 'string' ||
    !Array.isArray(submission.annotations) ||
    !submission.annotations.every(isAnnotation)
  ) {
    return { ok: false, reason: 'The review is not valid' }
  }
  const text = composeReview(submission)
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
