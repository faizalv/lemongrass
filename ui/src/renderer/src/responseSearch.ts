// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

// Search inside a parsed JSON response for the Rester Client's tree. A query is one term: plain
// text, key:value, a dotted path with * for any one key or index, or a comparison such as price>100.

export type PathSegment = string | number

// Which part of a node matched: its key name, its value, or both.
export interface SearchMatch {
  path: PathSegment[]
  key: boolean
  value: boolean
}

export interface SearchResult {
  // In document order, the order next and previous step through.
  matches: SearchMatch[]
  byPath: Map<string, SearchMatch>
  // Every proper ancestor of a match, as a pathId.
  ancestors: Set<string>
}

// A key named anywhere, or a dotted path from the root.
type Selector = { name: string } | { segments: string[] }

type Operator = '>' | '>=' | '<' | '<=' | '=' | '!='

export type SearchQuery =
  | { kind: 'text'; text: string }
  | { kind: 'keyValue'; selector: Selector; text: string }
  | { kind: 'path'; segments: string[]; fallback: string }
  | { kind: 'compare'; selector: Selector; op: Operator; operand: string }

export function pathId(path: PathSegment[]): string {
  return JSON.stringify(path)
}

function selectorOf(text: string): Selector {
  const trimmed = text.replace(/^\./, '')
  return trimmed.includes('.') ? { segments: trimmed.split('.') } : { name: trimmed }
}

function unquote(text: string): string {
  const t = text.trim()
  return t.length >= 2 && t.startsWith('"') && t.endsWith('"') ? t.slice(1, -1) : t
}

export function parseQuery(input: string): SearchQuery | null {
  const query = input.trim()
  if (!query) return null
  if (query.length >= 2 && query.startsWith('"') && query.endsWith('"')) {
    return { kind: 'text', text: query.slice(1, -1) }
  }
  const compare = query.match(/^([^\s<>=!:"]+)\s*(>=|<=|!=|>|<|=)\s*(.+)$/)
  if (compare) {
    return {
      kind: 'compare',
      selector: selectorOf(compare[1]),
      op: compare[2] as Operator,
      operand: unquote(compare[3])
    }
  }
  const keyValue = query.match(/^([^\s:"]+):(.*)$/)
  if (keyValue) {
    return { kind: 'keyValue', selector: selectorOf(keyValue[1]), text: unquote(keyValue[2]) }
  }
  if (query.includes('.') && !/\s/.test(query)) {
    return { kind: 'path', segments: query.replace(/^\./, '').split('.'), fallback: query }
  }
  return { kind: 'text', text: query }
}

function isBranch(value: unknown): value is Record<string, unknown> | unknown[] {
  return value !== null && typeof value === 'object'
}

// A primitive's text as the tree shows it, but a string without its quotes.
function valueText(value: unknown): string {
  return typeof value === 'string' ? value : JSON.stringify(value)
}

function contains(haystack: string, needle: string): boolean {
  return haystack.toLowerCase().includes(needle.toLowerCase())
}

function childrenOf(value: unknown): [PathSegment, unknown][] {
  if (Array.isArray(value)) return value.map((child, i) => [i, child])
  if (isBranch(value)) return Object.entries(value)
  return []
}

interface Node {
  path: PathSegment[]
  // The key it sits under in an object, absent for an array element and the root.
  name?: string
  value: unknown
}

// Every node in document order.
function* walk(value: unknown, path: PathSegment[] = [], name?: string): Generator<Node> {
  yield { path, name, value }
  for (const [segment, child] of childrenOf(value)) {
    yield* walk(child, [...path, segment], typeof segment === 'string' ? segment : undefined)
  }
}

// The nodes at a dotted path, * standing for any one key or index.
function nodesAt(root: unknown, segments: string[]): Node[] {
  let level: Node[] = [{ path: [], value: root }]
  for (const segment of segments) {
    const next: Node[] = []
    for (const node of level) {
      for (const [key, child] of childrenOf(node.value)) {
        if (segment !== '*' && String(key).toLowerCase() !== segment.toLowerCase()) continue
        next.push({
          path: [...node.path, key],
          name: typeof key === 'string' ? key : undefined,
          value: child
        })
      }
    }
    level = next
  }
  return level
}

function selected(root: unknown, selector: Selector): Node[] {
  if ('segments' in selector) return nodesAt(root, selector.segments)
  const name = selector.name.toLowerCase()
  return [...walk(root)].filter((node) => node.name?.toLowerCase() === name)
}

function compareValue(value: unknown, op: Operator, operand: string): boolean {
  if (isBranch(value)) return false
  const left = typeof value === 'number' ? value : Number(valueText(value))
  const right = Number(operand)
  const numeric = operand.trim() !== '' && !Number.isNaN(right) && !Number.isNaN(left)
  if (numeric) {
    switch (op) {
      case '>':
        return left > right
      case '>=':
        return left >= right
      case '<':
        return left < right
      case '<=':
        return left <= right
      case '=':
        return left === right
      case '!=':
        return left !== right
    }
  }
  const equal = valueText(value).toLowerCase() === operand.toLowerCase()
  if (op === '=') return equal
  if (op === '!=') return !equal
  return false
}

function matchesOf(root: unknown, query: SearchQuery): SearchMatch[] {
  switch (query.kind) {
    case 'text':
      return [...walk(root)].flatMap((node) => {
        const key = node.name !== undefined && contains(node.name, query.text)
        const value = !isBranch(node.value) && contains(valueText(node.value), query.text)
        return key || value ? [{ path: node.path, key, value }] : []
      })
    case 'keyValue':
      return selected(root, query.selector).flatMap((node): SearchMatch[] => {
        const key = node.name !== undefined
        if (isBranch(node.value)) return query.text ? [] : [{ path: node.path, key, value: false }]
        if (!contains(valueText(node.value), query.text)) return []
        return [{ path: node.path, key, value: true }]
      })
    case 'path':
      return nodesAt(root, query.segments).map((node) => ({
        path: node.path,
        key: node.name !== undefined,
        value: !isBranch(node.value)
      }))
    case 'compare':
      return selected(root, query.selector)
        .filter((node) => compareValue(node.value, query.op, query.operand))
        .map((node) => ({ path: node.path, key: node.name !== undefined, value: true }))
  }
}

// Runs query over root. A path that matches nothing is searched again as plain text, so a query
// such as example.com still finds that text.
export function searchJson(root: unknown, query: SearchQuery): SearchResult {
  let matches = matchesOf(root, query)
  if (query.kind === 'path' && matches.length === 0) {
    matches = matchesOf(root, { kind: 'text', text: query.fallback })
  }
  const byPath = new Map<string, SearchMatch>()
  const ancestors = new Set<string>()
  for (const match of matches) {
    byPath.set(pathId(match.path), match)
    for (let i = 0; i < match.path.length; i++) ancestors.add(pathId(match.path.slice(0, i)))
  }
  return { matches, byPath, ancestors }
}
