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
