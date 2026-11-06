// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// A box head never wraps and scrolls sideways when the box is narrow. A mouse wheel only sends
// vertical deltas, so a plain vertical turn over an overflowing head scrolls it sideways instead.
export function wheelSideways(event: WheelEvent): void {
  const area = event.currentTarget as HTMLElement
  if (event.deltaX !== 0 || event.shiftKey || area.scrollWidth <= area.clientWidth) return
  area.scrollLeft += event.deltaY
  event.preventDefault()
}
