// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// The resolver's icons in one map, as GitIcon holds the git panel's, so a control is a name here
// and never its own inline SVG. Each is stroke path data in a 14 by 14 box.
export const CONFLICT_ICONS = {
  prev: ['M3.5 8.5L7 5l3.5 3.5'],
  next: ['M3.5 5.5L7 9l3.5-3.5'],
  // Two double chevrons meeting in the middle, as in compressing the file to its conflicts.
  focus: ['M4 1.5l3 2 3-2', 'M4 4l3 2 3-2', 'M4 10l3-2 3 2', 'M4 12.5l3-2 3 2'],
  more: [
    'M3 6.3a.7.7 0 1 1 0 1.4a.7.7 0 1 1 0-1.4',
    'M7 6.3a.7.7 0 1 1 0 1.4a.7.7 0 1 1 0-1.4',
    'M11 6.3a.7.7 0 1 1 0 1.4a.7.7 0 1 1 0-1.4'
  ],
  accept: ['M3 7.5l2.5 2.5L11 4.5'],
  // A take points from its side into the middle, as the decision gutter's chevron does: the left
  // ref's to the right, the right ref's to the left, and both meet. conflictEditor.ts keeps its
  // own copy of the two chevrons for the gutter.
  takeLeft: ['M3 3.5l3.5 3.5L3 10.5M7.5 3.5L11 7l-3.5 3.5'],
  takeRight: ['M11 3.5L7.5 7l3.5 3.5M6.5 3.5L3 7l3.5 3.5'],
  takeBoth: ['M2.5 3.5L6 7l-3.5 3.5', 'M11.5 3.5L8 7l3.5 3.5'],
  reset: ['M2 7a5 5 0 1 0 1.46-3.54L2 4.9', 'M2 2v2.9h2.9'],
  reject: ['M4 4l6 6M10 4l-6 6'],
  // Between the side panes and the result: an arrow down from a side that was taken, and only a
  // cross for one that was not.
  arrowTaken: ['M7 1.5v11', 'M3.2 8.8L7 12.5l3.8-3.7'],
  crossRejected: ['M3.5 3.5l7 7M10.5 3.5l-7 7'],
  // A speech bubble: asking the model about the block under the cursor.
  ask: [
    'M3 2.5h8a1.5 1.5 0 0 1 1.5 1.5v4.5a1.5 1.5 0 0 1-1.5 1.5H6.5L4 12V10H3a1.5 1.5 0 0 1-1.5-1.5V4A1.5 1.5 0 0 1 3 2.5z'
  ],
  // Two speech bubbles, one behind the other: the thread with the agent, as against asking once.
  thread: [
    'M2.5 1.5h6a1 1 0 0 1 1 1v3.5a1 1 0 0 1-1 1H5L3 8.5V7h-.5a1 1 0 0 1-1-1V2.5a1 1 0 0 1 1-1z',
    'M11.5 5.5a1 1 0 0 1 1 1V10a1 1 0 0 1-1 1H11v1.5L9 11H6a1 1 0 0 1-1-1V9'
  ],
  // Ticked rows: the decisions to make. Three columns: the three panes resolved by hand.
  decisions: ['M1.5 3.5l1.2 1.2L5 2.5', 'M7 3.5h5.5', 'M1.5 9.5l1.2 1.2L5 8.5', 'M7 9.5h5.5'],
  manual: ['M1.5 2.5h11v9h-11z', 'M5.2 2.5v9', 'M8.8 2.5v9']
} as const satisfies Record<string, readonly string[]>

export type ConflictIconName = keyof typeof CONFLICT_ICONS
