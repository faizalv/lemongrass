// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { lstatSync, readdirSync } from 'fs'
import { join } from 'path'

// A folder's own mtime moves only when entries are added, removed or renamed, so an edit to a file inside it is found
// only by walking everything below. lstat keeps a symlink loop from recursing forever.
export function latestActivityMs(path: string): number {
  let stats
  try {
    stats = lstatSync(path)
  } catch {
    return 0
  }
  let newest = stats.mtimeMs
  if (!stats.isDirectory()) return newest
  for (const entry of readdirSync(path)) {
    newest = Math.max(newest, latestActivityMs(join(path, entry)))
  }
  return newest
}

// Task folders most recently active first, so a task picked up again rises to the top; `pinned` stays first.
export function byLatestActivity(dir: string, names: string[], pinned: string): string[] {
  const activity = new Map(
    names.map((name) => [name, name === pinned ? 0 : latestActivityMs(join(dir, name))])
  )
  return [...names].sort((a, b) => {
    if (a === pinned) return -1
    if (b === pinned) return 1
    return activity.get(b)! - activity.get(a)! || a.localeCompare(b)
  })
}
