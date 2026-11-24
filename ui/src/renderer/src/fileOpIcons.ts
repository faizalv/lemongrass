// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Menu icons for the file operations both sidebar trees offer, so Cut or Delete looks the same in
// the Files and Biblio trees.
export const FILE_OP_ICONS = {
  cut: [
    'M3.5 12a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3ZM10.5 12a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3Z',
    'M4.6 9.3 10 2M9.4 9.3 4 2'
  ],
  copy: ['M4.5 4.5h7v8h-7z', 'M9.5 4.5V2H2v8h2.5'],
  paste: ['M4.5 2.5H3v10h8v-10H9.5', 'M5 1.5h4v2H5z'],
  path: [
    'M6 8a2.5 2.5 0 0 0 3.5 0l2-2a2.5 2.5 0 0 0-3.5-3.5l-.7.7',
    'M8 6a2.5 2.5 0 0 0-3.5 0l-2 2a2.5 2.5 0 0 0 3.5 3.5l.7-.7'
  ],
  trash: ['M2.5 3.5h9', 'M5.5 3.5V2h3v1.5', 'M3.5 3.5l.7 9h5.6l.7-9']
} as const
