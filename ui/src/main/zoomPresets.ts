// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

// The panes and the biblio column need about this many CSS pixels of width, so a zoom that
// would leave the page narrower than this on the window's screen is not offered.
const MIN_LAYOUT_WIDTH = 1280

// Zoom factors in quarter steps from 75% up to the largest that keeps MIN_LAYOUT_WIDTH on a work
// area this wide (DIPs), never capped below 100%. Quarters are exact in binary, so callers can
// compare factors with ===.
export function zoomPresets(workAreaWidth: number): number[] {
  const maxQuarters = Math.max(4, Math.floor((workAreaWidth / MIN_LAYOUT_WIDTH) * 4))
  const presets: number[] = []
  for (let quarters = 3; quarters <= maxQuarters; quarters++) presets.push(quarters / 4)
  return presets
}
