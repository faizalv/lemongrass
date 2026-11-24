// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { posix } from 'path'

// The convention's fixed folders: tools and every session find biblio content through them, so
// the tree's file operations may paste into them but never cut, copy or delete them.
const CATEGORY_PATHS = new Set(['books', 'handover', 'laws', 'scratchpad', 'scratchpad/archive'])

// relPath is biblio-relative and '/'-separated. It is normalized first, the way resolve() will
// read it, so a spelling such as "books/", "./laws" or "scratchpad/x/.." cannot pass the lookup.
export function isBiblioCategory(relPath: string): boolean {
  const normal = posix.normalize(relPath).replace(/^(\.\/)+|\/+$/g, '')
  return normal === '' || normal === '.' || CATEGORY_PATHS.has(normal)
}
