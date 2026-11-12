// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { showDragGhost } from '../../../dragGhost'

// An entry dragged out of one server tab's Files layer toward another's. The drag carries its own
// MIME type so a layer tells it from a desktop file drop. During dragover the data is unreadable,
// so the entry is read from here; a drop reads the data, since the ghost's end clears this on the
// window's capturing drop, before the layer's own handler runs.

export const ENTRY_MIME = 'application/x-lemongrass-ssh-entry'

export interface FileDrag {
  tabId: string
  alias: string
  path: string
  name: string
  dir: boolean
  size: number
}

let current: FileDrag | null = null

export function currentFileDrag(): FileDrag | null {
  return current
}

export function startFileDrag(event: DragEvent, el: HTMLElement, drag: FileDrag): void {
  if (!event.dataTransfer) return
  event.dataTransfer.effectAllowed = 'copy'
  event.dataTransfer.setData(ENTRY_MIME, JSON.stringify(drag))
  showDragGhost(event, el, { background: 'var(--color-surface-1)', onEnd: () => (current = null) })
  current = drag
}

export function isEntryDrag(event: DragEvent): boolean {
  return event.dataTransfer?.types.includes(ENTRY_MIME) ?? false
}

export function droppedEntry(event: DragEvent): FileDrag | null {
  const raw = event.dataTransfer?.getData(ENTRY_MIME)
  if (!raw) return null
  try {
    return JSON.parse(raw) as FileDrag
  } catch {
    return null
  }
}
