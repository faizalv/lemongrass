// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import { TOOL_KINDS, toolKindById, type ToolId, type ToolKind } from './toolKinds'

// Pins are app-wide, a layout preference like the sidebar view. The dock is a set of stateless
// shortcuts: it shows only what is pinned, in pin order, and no state.
export const toolDock = reactive({ pinned: [] as ToolId[], drawerOpen: false })

let loaded = false

export async function loadToolDock(): Promise<void> {
  if (loaded) return
  loaded = true
  const saved = await window.api.uiState.load()
  toolDock.pinned = (saved.pinnedTools ?? []).filter((id): id is ToolId => !!toolKindById(id))
}

export function isAlwaysPinned(id: ToolId): boolean {
  return !!toolKindById(id)?.alwaysPinned
}

export function isPinned(id: ToolId): boolean {
  return isAlwaysPinned(id) || toolDock.pinned.includes(id)
}

export function togglePin(id: ToolId): void {
  if (isAlwaysPinned(id)) return
  toolDock.pinned = toolDock.pinned.includes(id)
    ? toolDock.pinned.filter((entry) => entry !== id)
    : [...toolDock.pinned, id]
  window.api.uiState.save({ pinnedTools: [...toolDock.pinned] })
}

// Always-pinned tools first in registry order, then the human's pins in pin order.
export function pinnedTools(): ToolKind[] {
  const fixed = TOOL_KINDS.filter((kind) => kind.alwaysPinned)
  const chosen = toolDock.pinned
    .map((id) => toolKindById(id))
    .filter((kind): kind is ToolKind => !!kind && !kind.alwaysPinned)
  return [...fixed, ...chosen]
}
