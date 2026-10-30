// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import type { GitConflictOperation, GitStatus } from '../../preload/types'

export const CONFLICT_SCOPE_PREFIX = 'conflict:'

export function conflictScope(
  operation: GitConflictOperation,
  leftRef: string,
  rightRef: string
): string {
  return `${CONFLICT_SCOPE_PREFIX}${operation}:${leftRef}:${rightRef}`
}

const OPERATIONS: readonly string[] = ['merge', 'rebase', 'stash']

// Git refs cannot contain a colon, so the key splits back into exactly its three parts.
export function parseConflictScope(
  scope: string
): { operation: GitConflictOperation; leftRef: string; rightRef: string } | null {
  if (!scope.startsWith(CONFLICT_SCOPE_PREFIX)) return null
  const parts = scope.slice(CONFLICT_SCOPE_PREFIX.length).split(':')
  if (parts.length !== 3 || !OPERATIONS.includes(parts[0])) return null
  const [operation, leftRef, rightRef] = parts
  return { operation: operation as GitConflictOperation, leftRef, rightRef }
}

// undefined while the status is not loaded yet, so nothing is cleared on a guess. Conflicted
// files with no stopped operation are a stash reapply, which GitStatus.operation leaves out.
export function activeConflictKind(
  status: GitStatus | null
): GitConflictOperation | null | undefined {
  if (!status) return undefined
  if (status.operation) return status.operation
  return status.files.some((file) => file.group === 'conflicted') ? 'stash' : null
}

// The conflict scopes that ended: all of them once nothing is stopped, else those of another kind.
export function endedConflictScopes(scopes: string[], kind: GitConflictOperation | null): string[] {
  const live = kind ? `${CONFLICT_SCOPE_PREFIX}${kind}:` : null
  return scopes.filter(
    (scope) => scope.startsWith(CONFLICT_SCOPE_PREFIX) && (!live || !scope.startsWith(live))
  )
}
