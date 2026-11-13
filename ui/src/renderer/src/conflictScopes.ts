// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { GitConflictOperation, GitStatus } from '../../preload/types'

// undefined while the status is not loaded yet, so nothing is cleared on a guess. Conflicted
// files with no stopped operation are a stash reapply, which GitStatus.operation leaves out.
export function activeConflictKind(
  status: GitStatus | null
): GitConflictOperation | null | undefined {
  if (!status) return undefined
  if (status.operation) return status.operation
  return status.files.some((file) => file.group === 'conflicted') ? 'stash' : null
}
