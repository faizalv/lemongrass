// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

import type { Directive } from 'vue'

// A mouse wheel only scrolls vertically, so a horizontal strip such as a row of tabs would never
// move under it. v-wheel-scroll-x turns a mostly vertical wheel turn into horizontal scroll while
// the element can scroll sideways, and leaves every other wheel event alone: a trackpad's own
// sideways swipe, and any wheel over a strip that fits, which then scrolls the page as usual.

const LINE_PX = 16

// The horizontal distance a wheel event should scroll, or null to leave the event to the browser.
// deltaMode 1 counts lines, as some mice on Linux report, and 2 counts pages of the strip's width.
export function wheelScrollX(
  event: Pick<WheelEvent, 'deltaX' | 'deltaY' | 'deltaMode'>,
  clientWidth: number,
  scrollWidth: number
): number | null {
  if (scrollWidth <= clientWidth) return null
  if (Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return null
  if (event.deltaMode === 1) return event.deltaY * LINE_PX
  if (event.deltaMode === 2) return event.deltaY * clientWidth
  return event.deltaY
}

const listeners = new WeakMap<HTMLElement, (event: WheelEvent) => void>()

export const vWheelScrollX: Directive<HTMLElement> = {
  mounted(el) {
    const onWheel = (event: WheelEvent): void => {
      const dx = wheelScrollX(event, el.clientWidth, el.scrollWidth)
      if (dx === null) return
      event.preventDefault()
      el.scrollLeft += dx
    }
    listeners.set(el, onWheel)
    // Not passive, since preventDefault keeps the page from scrolling under the strip.
    el.addEventListener('wheel', onWheel, { passive: false })
  },
  unmounted(el) {
    const onWheel = listeners.get(el)
    if (onWheel) el.removeEventListener('wheel', onWheel)
    listeners.delete(el)
  }
}
