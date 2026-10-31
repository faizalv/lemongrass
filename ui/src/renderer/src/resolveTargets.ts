// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import type { LayoutLeaf } from './layoutTree'

export interface ResolveTarget {
  tabId: string
  label: string
  paneId: string
  // The tab is the one its pane shows, so the human can see it without switching tabs.
  visible: boolean
}

interface ShellLike {
  id: string
  kind: string
  command?: string
  label?: string
}

// Only agents that read a typed prompt can take a conflict; a plain shell would run it as commands.
const AGENT_COMMANDS = new Set(['claude', 'codex'])

function isAgent(tab: ShellLike): boolean {
  return tab.kind === 'shell' && AGENT_COMMANDS.has(tab.command ?? '')
}

// The tab picker's order: the tab each pane shows, in pane order, then every other Claude or Codex
// tab in layout order. With many tabs open the ones already on screen are the likely picks.
export function orderTargets(
  leaves: LayoutLeaf<ShellLike>[],
  titles: Record<string, string | undefined>
): ResolveTarget[] {
  const target = (leaf: LayoutLeaf<ShellLike>, tab: ShellLike): ResolveTarget => ({
    tabId: tab.id,
    label: titles[tab.id] ?? tab.label ?? tab.id,
    paneId: leaf.id,
    visible: leaf.activeTabId === tab.id
  })
  const visible = leaves.flatMap((leaf) =>
    leaf.tabs
      .filter((tab) => tab.id === leaf.activeTabId && isAgent(tab))
      .map((tab) => target(leaf, tab))
  )
  const rest = leaves.flatMap((leaf) =>
    leaf.tabs
      .filter((tab) => tab.id !== leaf.activeTabId && isAgent(tab))
      .map((tab) => target(leaf, tab))
  )
  return [...visible, ...rest]
}
