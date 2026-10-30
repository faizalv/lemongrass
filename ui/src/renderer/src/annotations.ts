// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import type { Annotation, AnnotationAnchor, ReviewScope } from '../../preload/types'
import type { ProjectRef } from './workspace'

// Pending reviews, per project and then per scope. A scope names what the comments belong to,
// such as one stopped merge, and its annotator clears it when that ends. Nothing here reads an
// anchor, so every annotator kind shares this store.
export interface PendingReview {
  annotations: Annotation[]
  message: string
}

const reviews = reactive<Record<string, Record<string, PendingReview>>>({})

const EMPTY: PendingReview = Object.freeze({ annotations: [], message: '' }) as PendingReview

// Reading never creates an entry, so a render can call this without writing to the store.
export function reviewOf(projectId: string, scope: string): PendingReview {
  return reviews[projectId]?.[scope] ?? EMPTY
}

function ensure(projectId: string, scope: string): PendingReview {
  const scopes = (reviews[projectId] ??= {})
  return (scopes[scope] ??= { annotations: [], message: '' })
}

export function addAnnotation(
  projectId: string,
  scope: string,
  anchor: AnnotationAnchor,
  quote: string,
  comment: string
): string {
  const id = crypto.randomUUID()
  ensure(projectId, scope).annotations.push({ id, anchor, quote, comment })
  return id
}

export function editAnnotation(
  projectId: string,
  scope: string,
  id: string,
  comment: string
): void {
  const annotation = reviews[projectId]?.[scope]?.annotations.find((a) => a.id === id)
  if (annotation) annotation.comment = comment
}

export function removeAnnotation(projectId: string, scope: string, id: string): void {
  const review = reviews[projectId]?.[scope]
  if (!review) return
  const index = review.annotations.findIndex((a) => a.id === id)
  if (index >= 0) review.annotations.splice(index, 1)
}

export function setReviewMessage(projectId: string, scope: string, message: string): void {
  ensure(projectId, scope).message = message
}

export function clearReview(projectId: string, scope: string): void {
  const scopes = reviews[projectId]
  if (scopes) delete scopes[scope]
}

// Clears every scope of one annotator kind, such as all conflict scopes once no merge is stopped.
export function clearScopes(projectId: string, prefix: string): void {
  const scopes = reviews[projectId]
  if (!scopes) return
  for (const scope of Object.keys(scopes)) {
    if (scope.startsWith(prefix)) delete scopes[scope]
  }
}

export function scopesOf(projectId: string): string[] {
  return Object.keys(reviews[projectId] ?? {})
}

export function projectsWithReviews(): string[] {
  return Object.keys(reviews).filter((projectId) => Object.keys(reviews[projectId]).length > 0)
}

export type ReviewPrepared = { ok: true; scope: ReviewScope } | { ok: false; error: string }

type ReviewPreparer = (project: ProjectRef, scope: string) => Promise<ReviewPrepared>

const preparers: { prefix: string; prepare: ReviewPreparer }[] = []

// Each annotator kind builds the scope its reviews are sent with, and may save work first, so the
// bar sends a review without knowing what its scope holds.
export function registerReviewPreparer(prefix: string, prepare: ReviewPreparer): void {
  preparers.push({ prefix, prepare })
}

export function prepareReview(project: ProjectRef, scope: string): Promise<ReviewPrepared> {
  const preparer = preparers.find((entry) => scope.startsWith(entry.prefix))
  if (!preparer) return Promise.resolve({ ok: false, error: `No review is set up for ${scope}.` })
  return preparer.prepare(project, scope)
}
