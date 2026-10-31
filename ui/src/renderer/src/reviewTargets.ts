// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { allLeaves } from './layoutTree'
import { orderTargets, type ResolveTarget } from './resolveTargets'
import { shellTitles } from './shellRegistry'
import { workspaceOf } from './workspace'

export interface ReviewTarget {
  tabId: string
  label: string
}

// Only agents that read a typed prompt can take a review; a plain shell would run it as commands.
const REVIEW_COMMANDS = new Set(['claude', 'codex'])

export function reviewTargetsOf(projectId: string): ReviewTarget[] {
  const root = workspaceOf(projectId)?.layout.root ?? null
  return allLeaves(root)
    .flatMap((leaf) => leaf.tabs)
    .flatMap((tab) =>
      tab.kind === 'shell' && REVIEW_COMMANDS.has(tab.command)
        ? [{ tabId: tab.id, label: shellTitles[tab.id] ?? tab.label }]
        : []
    )
}

// The Claude and Codex tabs in picker order, the ones each pane shows first.
export function resolveTargetsOf(projectId: string): ResolveTarget[] {
  return orderTargets(allLeaves(workspaceOf(projectId)?.layout.root ?? null), shellTitles)
}
