// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// A dragged element is drawn as an in-page ghost instead of Chromium's drag image. The native
// image is a snapshot of the element's bounding rectangle, and on Linux it has no alpha, so the
// corners outside a rounded element render black. The native image is replaced by a transparent
// pixel and a clone follows the pointer, keeping the grab offset.

const BLANK = new Image()
BLANK.src = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7'

let ghost: HTMLElement | null = null
let onEnd: (() => void) | null = null
let grabX = 0
let grabY = 0

export interface DragGhostOptions {
  // A fill for an element that has none at rest, so the clone does not float as bare text.
  background?: string
  // Runs once when the drag ends, by a drop anywhere or the source's dragend.
  onEnd?: () => void
}

export function showDragGhost(
  event: DragEvent,
  el: HTMLElement,
  opts: DragGhostOptions = {}
): void {
  hideDragGhost()
  onEnd = opts.onEnd ?? null
  event.dataTransfer?.setDragImage(BLANK, 0, 0)

  const rect = el.getBoundingClientRect()
  grabX = event.clientX - rect.left
  grabY = event.clientY - rect.top
  ghost = el.cloneNode(true) as HTMLElement
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
  if (opts.background) ghost.style.background = opts.background
  document.body.appendChild(ghost)

  window.addEventListener('dragover', follow, true)
  window.addEventListener('drop', hideDragGhost, true)
  // dragend fires on the source element even after a drop unmounted it, but never reaches window
  // in that case, so it is bound to the element itself.
  el.addEventListener('dragend', hideDragGhost, { once: true })
}

function follow(event: DragEvent): void {
  if (!ghost) return
  ghost.style.left = `${event.clientX - grabX}px`
  ghost.style.top = `${event.clientY - grabY}px`
}

export function hideDragGhost(): void {
  ghost?.remove()
  ghost = null
  window.removeEventListener('dragover', follow, true)
  window.removeEventListener('drop', hideDragGhost, true)
  const end = onEnd
  onEnd = null
  end?.()
}
