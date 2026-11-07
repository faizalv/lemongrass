// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { WorkspaceTab } from '../../preload/types'

// The tab kinds this build can render. The Record over the kind union makes the compiler reject
// a missing or extra name, so the list cannot drift from WorkspaceTab. It lives apart from
// tabKinds.ts because that module imports every view, and workspace.ts, which every view imports,
// needs this check when it restores a saved layout.
const KINDS: Record<WorkspaceTab['kind'], true> = {
  doc: true,
  file: true,
  shell: true,
  diff: true,
  conflict: true,
  history: true,
  connector: true,
  web: true,
  workgroup: true,
  device: true,
  resterClient: true,
  ssh: true,
  sshServer: true
}

export function isTabKind(kind: string): kind is WorkspaceTab['kind'] {
  return Object.hasOwn(KINDS, kind)
}
