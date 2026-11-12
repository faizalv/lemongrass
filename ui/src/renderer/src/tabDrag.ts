// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { showDragGhost } from './dragGhost'

// The tab drag in progress, drawn by the shared drag ghost.

export type TabDrag = { paneId: string; tabId: string }

let current: TabDrag | null = null

// The drag in progress, readable during dragover, where the drag data itself is not.
export function currentTabDrag(): TabDrag | null {
  return current
}

export function startTabDrag(event: DragEvent, tabEl: HTMLElement, drag: TabDrag): void {
  // An inactive tab has no fill at rest, so the ghost takes the hover fill it had when grabbed.
  const background = tabEl.classList.contains('active') ? undefined : 'var(--color-surface-1)'
  showDragGhost(event, tabEl, { background, onEnd: () => (current = null) })
  current = drag
}
