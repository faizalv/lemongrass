// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { IconShape } from '../../../tabKinds'

// The Files drawer's glyphs in the app's 14 by 14 stroke style. New folder and Rename match the
// project explorer's so the same action looks the same in both trees.

function shapes(...paths: string[]): IconShape[] {
  return paths.map((d) => ({ tag: 'path', attrs: { d } }))
}

export const DRAWER_ICONS = {
  files: shapes('M1.5 3.5h4l1.5 2h5.5v6h-11z'),
  up: shapes('M7 11.5v-9', 'M3.5 6 7 2.5 10.5 6'),
  refresh: shapes('M11.5 7a4.5 4.5 0 1 1-1.3-3.2', 'M11.5 2v2.5H9'),
  newFolder: shapes('M7 11.5H1.5v-8h4l1.5 2h5.5v2', 'M10.5 9v4M8.5 11h4'),
  rename: shapes('M9.5 1.5 12.5 4.5 5 12H2V9z'),
  download: shapes('M7 2v7', 'M4 6.5l3 3 3-3', 'M2.5 12h9'),
  upload: shapes('M7 9.5V2.5', 'M4 5.5l3-3 3 3', 'M2.5 12h9')
}
