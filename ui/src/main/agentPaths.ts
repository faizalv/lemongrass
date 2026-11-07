// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { DepsReport } from '../preload/types'

// Where the dependency check found each agent CLI, set by deps.ts after each check. The check also looks in the
// vendors' install folders, which can be missing from the app's PATH, so a tab falls back to this path when the bare
// name is not found, and a row that reads ok always starts. Kept apart from deps.ts so pty.ts and its tests need no
// Electron.
let paths: Record<string, string> = {}

export function setAgentPaths(report: DepsReport): void {
  paths = Object.fromEntries(
    report.deps.filter((dep) => dep.agent && dep.path).map((dep) => [dep.name, dep.path])
  )
}

export function agentPath(command: string): string | undefined {
  return paths[command]
}
