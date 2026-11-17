// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Chromium's select-all covers every selectable node in the window, so one keystroke in a doc
// would also select the other panes' content. Here it selects only the region last clicked into.

const REGION = '.cm-content, .sk-selectable, pre'

let lastPointerTarget: Element | null = null

// Fields, editors and the terminal's hidden textarea keep their own select-all, and a terminal
// passes it to the shell as ^A.
function ownsSelectAll(element: Element | null): boolean {
  if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) return true
  return element instanceof HTMLElement && element.isContentEditable
}

function regionOf(element: Element | null): Element | null {
  if (!element || !element.isConnected) return null
  return element.closest(REGION)
}

function onPointerDown(event: PointerEvent): void {
  lastPointerTarget = event.target instanceof Element ? event.target : null
}

function onKeyDown(event: KeyboardEvent): void {
  const isMac = window.electron.process.platform === 'darwin'
  const modifier = isMac ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey
  if (!modifier || event.altKey || event.shiftKey || event.code !== 'KeyA') return
  if (ownsSelectAll(document.activeElement)) return
  event.preventDefault()
  const region = regionOf(lastPointerTarget)
  if (!region) return
  const range = document.createRange()
  range.selectNodeContents(region)
  const selection = window.getSelection()
  selection?.removeAllRanges()
  selection?.addRange(range)
}

export function installSelectAll(): void {
  window.addEventListener('pointerdown', onPointerDown, true)
  window.addEventListener('keydown', onKeyDown, true)
}
