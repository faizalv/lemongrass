// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { MatchRange } from './treeSearch'

export type BiblioScope = 'all' | 'books' | 'scratchpad' | 'handover' | 'laws'

export const BIBLIO_ARCHIVE = 'scratchpad/archive'

// Above this many matches the tree is not opened down to them, since the tree is not virtualized
// and opening most of a large archive at once stalls a slow machine for a moment.
export const BIBLIO_EXPAND_LIMIT = 200

export interface BiblioSearchNode {
  name: string
  path: string
  type: 'file' | 'dir'
  children?: BiblioSearchNode[]
}

export interface BiblioSearchResult<T extends BiblioSearchNode = BiblioSearchNode> {
  // The tree to show: every category row in its usual order, the ones in scope cut down to their
  // matches and the folders above them, siblings closest match first.
  tree: T[]
  // Matched paths with the ranges inside the shown name; order is the order they show in.
  matches: Map<string, MatchRange[]>
  order: string[]
  // Categories in scope with nothing matched.
  noMatch: Set<string>
  // Folders to open so every match shows; empty when the matches pass the expand limit.
  expand: Record<string, true>
  capped: boolean
}

// What a tree row shows: markdown files drop their suffix, so a match never lands on it.
export function biblioLabel(node: { name: string; type: 'file' | 'dir' }): string {
  return node.type === 'file' ? node.name.replace(/\.md$/, '') : node.name
}

export function ancestorsOf(path: string): string[] {
  const segments = path.split('/')
  const out: string[] = []
  for (let i = 1; i < segments.length; i++) out.push(segments.slice(0, i).join('/'))
  return out
}

function rangesOf(lower: string, needle: string): MatchRange[] {
  const ranges: MatchRange[] = []
  for (let at = lower.indexOf(needle); at !== -1; at = lower.indexOf(needle, at + needle.length)) {
    ranges.push([at, at + needle.length])
  }
  return ranges
}

function startsWord(label: string, index: number): boolean {
  if (index === 0) return true
  const before = label[index - 1]
  if ('-_. '.includes(before)) return true
  const here = label[index]
  return before === before.toLowerCase() && here !== here.toLowerCase()
}

// Closeness of a substring match, higher is closer: the exact name, then a name starting with the
// query, then the query starting a word, then anywhere inside. A shorter name wins a tie.
export function matchScore(label: string, needle: string, ranges: MatchRange[]): number {
  if (!ranges.length) return 0
  const lower = label.toLowerCase()
  let tier = 1
  if (lower === needle) tier = 4
  else if (ranges[0][0] === 0) tier = 3
  else if (ranges.some(([start]) => startsWord(label, start))) tier = 2
  return tier * 1000 - Math.min(label.length, 999)
}

interface Pruned<T> {
  node: T
  best: number
}

// Sorts closest first; Array.prototype.sort is stable, so equal scores keep tree order.
function byBest<T>(list: Pruned<T>[]): T[] {
  return list.sort((a, b) => b.best - a.best).map((entry) => entry.node)
}

// Cuts the loaded tree down to the substring matches of a query, in one walk with no IPC. The
// category rows and the archive are containers: they never match themselves, so a query such as
// "scratch" does not pull in every task. A matching folder keeps its whole content, matches first,
// so it can be opened to look around it.
export function searchBiblio<T extends BiblioSearchNode>(
  nodes: T[],
  query: string,
  scope: BiblioScope,
  includeArchive: boolean
): BiblioSearchResult<T> {
  const needle = query.trim().toLowerCase()
  const matches = new Map<string, MatchRange[]>()

  function prune(node: T): Pruned<T> | null {
    const label = biblioLabel(node)
    const ranges = needle ? rangesOf(label.toLowerCase(), needle) : []
    const own = matchScore(label, needle, ranges)
    if (own) matches.set(node.path, ranges)
    if (node.type !== 'dir') return own ? { node, best: own } : null

    const kids = (node.children ?? []) as T[]
    const kept: Pruned<T>[] = []
    const rest: T[] = []
    for (const child of kids) {
      const hit = prune(child)
      if (hit) kept.push(hit)
      else if (own) rest.push(child)
    }
    if (!own && !kept.length) return null
    const best = Math.max(own, ...kept.map((entry) => entry.best))
    return { node: { ...node, children: [...byBest(kept), ...rest] }, best }
  }

  function category(node: T): T {
    const kept: Pruned<T>[] = []
    let archive: T | null = null
    for (const child of (node.children ?? []) as T[]) {
      if (child.path === BIBLIO_ARCHIVE) {
        if (includeArchive) archive = category(child)
        continue
      }
      const hit = prune(child)
      if (hit) kept.push(hit)
    }
    const children = byBest(kept)
    // The archive comes after the live tasks so it never floods the top of the scratchpad.
    if (archive?.children?.length) children.push(archive)
    return { ...node, children }
  }

  const noMatch = new Set<string>()
  const tree = nodes.map((node) => {
    if (!needle || (scope !== 'all' && node.path !== scope)) return { ...node, children: [] }
    const cut = category(node)
    if (!cut.children?.length) noMatch.add(node.path)
    return cut
  })

  const order: string[] = []
  const walk = (list: BiblioSearchNode[]): void => {
    for (const node of list) {
      if (matches.has(node.path)) order.push(node.path)
      if (node.children) walk(node.children)
    }
  }
  walk(tree)

  const capped = order.length > BIBLIO_EXPAND_LIMIT
  const expand: Record<string, true> = {}
  if (!capped) for (const path of order) for (const dir of ancestorsOf(path)) expand[dir] = true
  return { tree, matches, order, noMatch, expand, capped }
}
