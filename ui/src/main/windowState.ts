// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { BrowserWindow, Rectangle, screen } from 'electron'
import { loadUiState, saveUiState } from './uiState'
import { zoomPresets } from './zoomPresets'

const DEFAULT_SIZE = { width: 900, height: 670 }

// Saved bounds that overlap no display, such as after a monitor was unplugged, would open the
// window off screen.
function onSomeDisplay(bounds: Rectangle): boolean {
  return screen
    .getAllDisplays()
    .some(
      ({ workArea }) =>
        bounds.x < workArea.x + workArea.width &&
        bounds.x + bounds.width > workArea.x &&
        bounds.y < workArea.y + workArea.height &&
        bounds.y + bounds.height > workArea.y
    )
}

export function savedWindowBounds(): Partial<Rectangle> {
  const saved = loadUiState().window
  if (!saved || !onSomeDisplay(saved)) return DEFAULT_SIZE
  return { x: saved.x, y: saved.y, width: saved.width, height: saved.height }
}

// The page is created at the saved zoom, so terminals never change size under an agent that
// already drew. Clamped like the renderer's restoreZoom, to the largest preset of the display the
// window opens on.
export function savedZoomFactor(bounds: Partial<Rectangle>): number {
  const { x, y, width, height } = bounds
  const display =
    x !== undefined && y !== undefined && width !== undefined && height !== undefined
      ? screen.getDisplayMatching({ x, y, width, height })
      : screen.getPrimaryDisplay()
  const presets = zoomPresets(display.workAreaSize.width)
  return Math.min(loadUiState().zoomFactor ?? 1, presets[presets.length - 1])
}

export function restoreWindowMode(win: BrowserWindow): void {
  const saved = loadUiState().window
  if (saved?.fullScreen) win.setFullScreen(true)
  else if (saved?.maximized) win.maximize()
}

// Saves the unmaximized bounds, so restoring a window that was maximized when it closed brings it
// back to the size it had before.
export function rememberWindowState(win: BrowserWindow): void {
  saveUiState({
    window: {
      ...win.getNormalBounds(),
      maximized: win.isMaximized(),
      fullScreen: win.isFullScreen()
    }
  })
}
