// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { IconShape } from '../../../tabKinds'

// The Files layer's glyphs as stroke path data in the app's 14 by 14 box, the form context menu
// items take; TabIcon draws them through shapesOf. New file, New folder, Rename, Copy and Delete
// match the project explorer's so the same action looks the same in both trees.
export const FILES_PATHS = {
  files: ['M1.5 3.5h4l1.5 2h5.5v6h-11z'],
  back: ['M8.5 3 4.5 7l4 4'],
  forward: ['M5.5 3l4 4-4 4'],
  reload: ['M11.5 7a4.5 4.5 0 1 1-1.3-3.2', 'M11.5 2v2.5H9'],
  paste: ['M4.5 2.5H3v10h8v-10H9.5', 'M5 1.5h4v2H5z'],
  newFolder: ['M7 11.5H1.5v-8h4l1.5 2h5.5v2', 'M10.5 9v4M8.5 11h4'],
  newFile: ['M8 1.5H3v11h4', 'M8 1.5 10.5 4v3', 'M8 1.5V4h2.5', 'M10.5 9v4M8.5 11h4'],
  upload: ['M7 9.5V2.5', 'M4 5.5l3-3 3 3', 'M2.5 12h9'],
  download: ['M7 2v7', 'M4 6.5l3 3 3-3', 'M2.5 12h9'],
  move: ['M1.5 3.5h4l1.5 2h5.5v6h-11z', 'M5.5 8.5h4M8 7l1.5 1.5L8 10'],
  copy: ['M4.5 4.5h7v8h-7z', 'M9.5 4.5V2H2v8h2.5'],
  rename: ['M9.5 1.5 12.5 4.5 5 12H2V9z'],
  trash: ['M2.5 3.5h9', 'M5.5 3.5V2h3v1.5', 'M3.5 3.5l.7 9h5.6l.7-9']
} as const

export function shapesOf(paths: readonly string[]): IconShape[] {
  return paths.map((d) => ({ tag: 'path', attrs: { d } }))
}
