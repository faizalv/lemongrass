// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Search Everywhere: file names and lines of code across the project and biblio/, opened from the
// header bar or a double Shift. Pure parts only; the dropdown is SearchEverywhere.vue.

import type { FuzzyMatch, MatchRange } from './treeSearch'
import type { BiblioNode, BiblioTree, ProjectTextHit } from '../../preload/types'

export type SearchScope = 'all' | 'files' | 'text'

export interface FileRow {
  kind: 'file'
  path: string
  name: string
  dir: string
  nameRanges: MatchRange[]
  score: number
}

export interface TextRow extends ProjectTextHit {
  kind: 'text'
}

export type SearchRow = FileRow | TextRow

// All shows a few names above the code lines, like JetBrains; Files alone shows more.
export const ALL_FILE_LIMIT = 10
export const FILES_LIMIT = 50
export const DOUBLE_SHIFT_MS = 350

// Every file in the biblio tree as a project-relative path.
export function biblioFilePaths(tree: BiblioTree | null): string[] {
  const out: string[] = []
  const visit = (nodes: BiblioNode[]): void => {
    for (const node of nodes) {
      if (node.type === 'file') out.push(`biblio/${node.path}`)
      else if (node.children) visit(node.children)
    }
  }
  if (tree?.toc) visit([tree.toc])
  if (tree) visit(tree.children)
  return out
}

// Ranks paths best first by `match`, the file explorer's `fuzzyMatch` (passed in so this module
// stays free of runtime imports for its tests).
export function rankFiles(
  paths: string[],
  query: string,
  limit: number,
  match: (path: string, query: string) => FuzzyMatch | null
): FileRow[] {
  const rows: FileRow[] = []
  for (const path of new Set(paths)) {
    const found = match(path, query)
    if (!found) continue
    const nameStart = path.lastIndexOf('/') + 1
    rows.push({
      kind: 'file',
      path,
      name: path.slice(nameStart),
      dir: nameStart === 0 ? '' : path.slice(0, nameStart - 1),
      nameRanges: found.ranges
        .filter(([, end]) => end > nameStart)
        .map(([start, end]): MatchRange => [
          Math.max(start, nameStart) - nameStart,
          end - nameStart
        ]),
      score: found.score
    })
  }
  rows.sort((a, b) => b.score - a.score || a.path.localeCompare(b.path))
  return rows.slice(0, limit)
}

export function rowsFor(scope: SearchScope, files: FileRow[], text: ProjectTextHit[]): SearchRow[] {
  const lines = text.map((hit): TextRow => ({ ...hit, kind: 'text' }))
  if (scope === 'files') return files.slice(0, FILES_LIMIT)
  if (scope === 'text') return lines
  return [...files.slice(0, ALL_FILE_LIMIT), ...lines]
}

// The last two segments of a path, the way a text row names its file.
export function shortPath(path: string): string {
  return path.split('/').slice(-2).join('/')
}

// Two presses of Shift on its own within DOUBLE_SHIFT_MS. Shift held for a capital letter or any
// other key in between never counts, and neither does key repeat.
export function createDoubleShift(
  trigger: () => void,
  now: () => number = () => performance.now()
): {
  keydown: (event: { key: string; repeat?: boolean }) => void
  keyup: (event: { key: string }) => void
  reset: () => void
} {
  let clean = false
  let lastUp = -Infinity
  return {
    keydown(event) {
      if (event.key !== 'Shift') {
        clean = false
        lastUp = -Infinity
      } else if (!event.repeat) clean = true
    },
    keyup(event) {
      if (event.key !== 'Shift' || !clean) return
      clean = false
      const at = now()
      if (at - lastUp <= DOUBLE_SHIFT_MS) {
        lastUp = -Infinity
        trigger()
      } else lastUp = at
    },
    reset() {
      clean = false
      lastUp = -Infinity
    }
  }
}
