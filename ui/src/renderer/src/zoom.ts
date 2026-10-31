// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'

// changes counts zooms the human made, so the indicator shows for those and not for the restore
// at start.
export const zoom = reactive({ factor: 1, presets: [1] as number[], changes: 0 })

export function zoomLabel(factor: number): string {
  return `${Math.round(factor * 100)}%`
}

function apply(factor: number): void {
  zoom.factor = factor
  window.electron.webFrame.setZoomFactor(factor)
}

export async function refreshZoomPresets(): Promise<void> {
  zoom.presets = await window.api.uiState.zoomPresets()
}

// A saved zoom above this screen's largest preset is shown at that preset but stays saved, so it
// comes back on a wider screen.
export async function restoreZoom(saved: number | undefined): Promise<void> {
  await refreshZoomPresets()
  apply(Math.min(saved ?? 1, zoom.presets[zoom.presets.length - 1]))
}

export function setZoom(factor: number): void {
  apply(factor)
  zoom.changes++
  window.api.uiState.save({ zoomFactor: factor })
}

// Moves to the next preset up or down, so the keyboard lands on the same values as the dropdown.
// At either end the indicator still shows, so the key press visibly did nothing.
export async function stepZoom(direction: 1 | -1): Promise<void> {
  await refreshZoomPresets()
  const next =
    direction > 0
      ? zoom.presets.find((preset) => preset > zoom.factor)
      : zoom.presets.findLast((preset) => preset < zoom.factor)
  if (next !== undefined) setZoom(next)
  else zoom.changes++
}
