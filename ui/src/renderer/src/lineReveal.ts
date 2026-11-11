// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'

// A request to scroll a file tab to a line and mark it, keyed by tab id. `seq` rises with every
// request, so asking for the same line again scrolls again. Kept outside the components because
// the tab's body may not be mounted yet when the request is made.
export interface LineReveal {
  line: number
  seq: number
}

const reveals = reactive(new Map<string, LineReveal>())
let seq = 0

export function revealLine(tabId: string, line: number): void {
  seq += 1
  reveals.set(tabId, { line, seq })
}

export function lineRevealOf(tabId: string): LineReveal | undefined {
  return reveals.get(tabId)
}

// Drops a request once a view has shown it, so showing the tab again later keeps the reader's own
// scroll position. A newer request for the tab is kept.
export function clearLineReveal(tabId: string, doneSeq: number): void {
  if (reveals.get(tabId)?.seq === doneSeq) reveals.delete(tabId)
}
