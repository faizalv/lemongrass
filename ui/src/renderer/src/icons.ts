// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { IconShape } from './tabKinds'

// Icon shapes used in more than one place, such as a tab and the empty state of its view. They
// live apart from tabKinds so a view can import one without importing the tab registry, which
// imports every view.
export const historyIcon: IconShape[] = [
  { tag: 'circle', attrs: { cx: 7, cy: 7, r: 5.2 } },
  { tag: 'path', attrs: { d: 'M7 4v3.2l2.2 1.3' } }
]

export const resterClientIcon: IconShape[] = [
  { tag: 'path', attrs: { d: 'M2 4.5h9.5L9.5 2.5' } },
  { tag: 'path', attrs: { d: 'M12 9.5H2.5l2 2' } }
]

export const decisionsIcon: IconShape[] = [
  { tag: 'path', attrs: { d: 'M2.5 2.5h9v6.5h-5l-2.5 2.5v-2.5h-1.5z' } },
  { tag: 'path', attrs: { d: 'M5 5.5l1.4 1.4L9 4.3' } }
]

// The shell tab's prompt inside a window frame: a terminal, on another machine.
export const sshIcon: IconShape[] = [
  { tag: 'rect', attrs: { x: 1.5, y: 2.5, width: 11, height: 9, rx: 1.5 } },
  { tag: 'path', attrs: { d: 'M4 5.75l1.75 1.5L4 8.75M7.25 9h2.75' } }
]

// A key: a round bow and a bit along the shaft.
export const keyIcon: IconShape[] = [
  { tag: 'circle', attrs: { cx: 4.5, cy: 7, r: 2.5 } },
  { tag: 'path', attrs: { d: 'M7 7h5.5M10.5 7v2M12.5 7v1.5' } }
]

export const vaultIcon: IconShape[] = [
  { tag: 'rect', attrs: { x: 2.5, y: 6.5, width: 9, height: 6, rx: 1.2 } },
  { tag: 'path', attrs: { d: 'M4.5 6.5V4.5a2.5 2.5 0 0 1 5 0v2' } },
  { tag: 'circle', attrs: { cx: 7, cy: 9.5, r: 0.8 } }
]

export const deviceIcon: IconShape[] = [
  { tag: 'rect', attrs: { x: 3.5, y: 1.5, width: 7, height: 11, rx: 1.4 } },
  { tag: 'path', attrs: { d: 'M6 10.5h2' } }
]

export const nodeIcon: IconShape[] = [
  { tag: 'circle', attrs: { cx: 7, cy: 7, r: 2 } },
  { tag: 'circle', attrs: { cx: 2.5, cy: 3, r: 1.2 } },
  { tag: 'circle', attrs: { cx: 11.5, cy: 3, r: 1.2 } },
  { tag: 'circle', attrs: { cx: 7, cy: 12, r: 1.2 } },
  { tag: 'path', attrs: { d: 'M3.5 3.8L5.5 5.8M10.5 3.8L8.5 5.8M7 9v1.8' } }
]

// Four tiles: every tool, as an app grid.
export const toolboxIcon: IconShape[] = [
  { tag: 'rect', attrs: { x: 1.5, y: 1.5, width: 4.5, height: 4.5, rx: 1 } },
  { tag: 'rect', attrs: { x: 8, y: 1.5, width: 4.5, height: 4.5, rx: 1 } },
  { tag: 'rect', attrs: { x: 1.5, y: 8, width: 4.5, height: 4.5, rx: 1 } },
  { tag: 'rect', attrs: { x: 8, y: 8, width: 4.5, height: 4.5, rx: 1 } }
]

export const pinIcon: IconShape[] = [
  { tag: 'path', attrs: { d: 'M5 1.5h4M6 1.5v4L4 8h6L8 5.5v-4M7 8v4.5' } }
]
