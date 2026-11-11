// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// A dragged tab is drawn as an in-page ghost instead of Chromium's drag image. The native image
// is a snapshot of the tab's bounding rectangle, and on Linux it has no alpha, so the corners
// outside a pill tab render black. The native image is replaced by a transparent pixel.

export type TabDrag = { paneId: string; tabId: string }

const BLANK = new Image()
BLANK.src = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7'

let current: TabDrag | null = null
let ghost: HTMLElement | null = null
let grabX = 0
let grabY = 0

// The drag in progress, readable during dragover, where the drag data itself is not.
export function currentTabDrag(): TabDrag | null {
  return current
}

export function startTabDrag(event: DragEvent, tabEl: HTMLElement, drag: TabDrag): void {
  endTabDrag()
  current = drag
  event.dataTransfer?.setDragImage(BLANK, 0, 0)

  const rect = tabEl.getBoundingClientRect()
  grabX = event.clientX - rect.left
  grabY = event.clientY - rect.top
  ghost = tabEl.cloneNode(true) as HTMLElement
  ghost.removeAttribute('draggable')
  Object.assign(ghost.style, {
    position: 'fixed',
    left: `${rect.left}px`,
    top: `${rect.top}px`,
    width: `${rect.width}px`,
    margin: '0',
    zIndex: '1000',
    pointerEvents: 'none',
    boxShadow: 'var(--shadow-md)',
    opacity: '0.92'
  })
  // An inactive tab has no fill at rest, so the ghost takes the hover fill it had when grabbed.
  if (!tabEl.classList.contains('active')) ghost.style.background = 'var(--color-surface-1)'
  document.body.appendChild(ghost)

  window.addEventListener('dragover', follow, true)
  window.addEventListener('drop', endTabDrag, true)
  // dragend fires on the source element even after a move unmounted it, but never reaches window
  // in that case, so it is bound to the element itself.
  tabEl.addEventListener('dragend', endTabDrag, { once: true })
}

function follow(event: DragEvent): void {
  if (!ghost) return
  ghost.style.left = `${event.clientX - grabX}px`
  ghost.style.top = `${event.clientY - grabY}px`
}

export function endTabDrag(): void {
  current = null
  ghost?.remove()
  ghost = null
  window.removeEventListener('dragover', follow, true)
  window.removeEventListener('drop', endTabDrag, true)
}
