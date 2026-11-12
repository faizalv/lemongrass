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

// A globe: a request to any address on the web, with no rester behind it.
export const plainRequestIcon: IconShape[] = [
  { tag: 'circle', attrs: { cx: 7, cy: 7, r: 5.2 } },
  { tag: 'path', attrs: { d: 'M7 1.8c-2.9 2.9-2.9 7.5 0 10.4M7 1.8c2.9 2.9 2.9 7.5 0 10.4' } },
  { tag: 'path', attrs: { d: 'M1.8 7h10.4' } }
]

export const decisionsIcon: IconShape[] = [
  { tag: 'path', attrs: { d: 'M2.5 2.5h9v6.5h-5l-2.5 2.5v-2.5h-1.5z' } },
  { tag: 'path', attrs: { d: 'M5 5.5l1.4 1.4L9 4.3' } }
]

// A server: two stacked boxes, each with a small light. Unlike the shell tab's prompt, so a
// login to another machine never reads as a local terminal.
export const sshIcon: IconShape[] = [
  { tag: 'rect', attrs: { x: 1.5, y: 2, width: 11, height: 4.5, rx: 1 } },
  { tag: 'rect', attrs: { x: 1.5, y: 7.5, width: 11, height: 4.5, rx: 1 } },
  { tag: 'path', attrs: { d: 'M3.75 4.25h1M3.75 9.75h1' } }
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

// A broadcast: the project shared out, in waves. Unlike the linked circles of Workgroup.
export const nodeIcon: IconShape[] = [
  { tag: 'circle', attrs: { cx: 7, cy: 7, r: 1.3 } },
  { tag: 'path', attrs: { d: 'M4.75 4.5a3.5 3.5 0 0 0 0 5M9.25 4.5a3.5 3.5 0 0 1 0 5' } },
  { tag: 'path', attrs: { d: 'M2.75 2.5a6.3 6.3 0 0 0 0 9M11.25 2.5a6.3 6.3 0 0 1 0 9' } }
]

// A folder: the sidebar's Biblio and Files tree.
export const filesIcon: IconShape[] = [
  {
    tag: 'path',
    attrs: {
      d: 'M1.5 3.5a1 1 0 0 1 1-1h3l1.5 1.5h4.5a1 1 0 0 1 1 1v6a1 1 0 0 1-1 1h-9a1 1 0 0 1-1-1z'
    }
  }
]
