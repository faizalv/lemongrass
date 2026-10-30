// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

// Search inside a parsed JSON response for the Rester Client's tree. A term is plain text,
// key:value, a dotted path with * for any one key or index, or a comparison such as price>100.
// Terms chain with AND, OR and NOT and group with parentheses; a chained query is judged per
// record, an object that is an element of an array, since two fields only count as together when
// one record holds both.

export type PathSegment = string | number

// Which part of a node matched: its key name, its value, or both.
export interface SearchMatch {
  path: PathSegment[]
  key: boolean
  value: boolean
}

export interface SearchResult {
  // The highlighted fields, in document order.
  matches: SearchMatch[]
  byPath: Map<string, SearchMatch>
  // What next and previous step through, in document order: the matches for a single term, the
  // qualifying records for a chained query.
  steps: PathSegment[][]
  unit: 'match' | 'record'
  // The qualifying records of a chained query, as pathIds.
  records: Set<string>
  // Every proper ancestor of a step or a highlighted field, as a pathId.
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
        if (!query.text && isBranch(node.value)) return [{ path: node.path, key, value: false }]
        // A list of plain values under the key, such as tags, matches by its elements.
        if (Array.isArray(node.value)) {
          return node.value.flatMap((element, i): SearchMatch[] =>
            !isBranch(element) && contains(valueText(element), query.text)
              ? [{ path: [...node.path, i], key: false, value: true }]
              : []
          )
        }
        if (isBranch(node.value)) return []
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

// The matches of one term. A path that matches nothing is searched again as plain text, so a
// query such as example.com still finds that text.
export function termMatches(root: unknown, query: SearchQuery): SearchMatch[] {
  const matches = matchesOf(root, query)
  if (query.kind === 'path' && matches.length === 0) {
    return matchesOf(root, { kind: 'text', text: query.fallback })
  }
  return matches
}

export function addAncestors(ancestors: Set<string>, path: PathSegment[]): void {
  for (let i = 0; i < path.length; i++) ancestors.add(pathId(path.slice(0, i)))
}

export function searchJson(root: unknown, query: SearchQuery): SearchResult {
  const matches = termMatches(root, query)
  const byPath = new Map<string, SearchMatch>()
  const ancestors = new Set<string>()
  for (const match of matches) {
    byPath.set(pathId(match.path), match)
    addAncestors(ancestors, match.path)
  }
  return {
    matches,
    byPath,
    steps: matches.map((m) => m.path),
    unit: 'match',
    records: new Set(),
    ancestors
  }
}

type Expr =
  | { kind: 'term'; query: SearchQuery }
  | { kind: 'and' | 'or'; left: Expr; right: Expr }
  | { kind: 'not'; expr: Expr }

type Token =
  | { kind: 'op'; op: 'AND' | 'OR' | 'NOT' }
  | { kind: 'open' | 'close' }
  | { kind: 'word'; text: string }

// Splits on whitespace outside double quotes, then peels grouping parentheses off each word.
function tokenize(input: string): Token[] {
  const words: string[] = []
  let word = ''
  let quoted = false
  for (const c of input) {
    if (c === '"') quoted = !quoted
    if (!quoted && /\s/.test(c)) {
      if (word) words.push(word)
      word = ''
    } else {
      word += c
    }
  }
  if (word) words.push(word)

  const tokens: Token[] = []
  for (const raw of words) {
    let w = raw
    while (w.startsWith('(')) {
      tokens.push({ kind: 'open' })
      w = w.slice(1)
    }
    let closes = 0
    while (w.endsWith(')') && !w.startsWith('"')) {
      closes++
      w = w.slice(0, -1)
    }
    const upper = w.toUpperCase()
    if (upper === 'AND' || upper === 'OR' || upper === 'NOT') tokens.push({ kind: 'op', op: upper })
    else if (w) tokens.push({ kind: 'word', text: w })
    for (let i = 0; i < closes; i++) tokens.push({ kind: 'close' })
  }
  return tokens
}

class ParseError extends Error {}

// or := and (OR and)* ; and := not (AND not)* ; not := NOT not | primary ; primary := ( or ) | term
function parse(tokens: Token[]): Expr {
  let at = 0
  const peek = (): Token | undefined => tokens[at]
  const isOp = (op: string): boolean => {
    const t = peek()
    return t?.kind === 'op' && t.op === op
  }

  function parseOr(): Expr {
    let left = parseAnd()
    while (isOp('OR')) {
      at++
      left = { kind: 'or', left, right: parseAnd() }
    }
    return left
  }
  function parseAnd(): Expr {
    let left = parseNot()
    while (isOp('AND')) {
      at++
      left = { kind: 'and', left, right: parseNot() }
    }
    return left
  }
  function parseNot(): Expr {
    if (isOp('NOT')) {
      at++
      return { kind: 'not', expr: parseNot() }
    }
    return parsePrimary()
  }
  function parsePrimary(): Expr {
    const t = peek()
    if (t?.kind === 'open') {
      at++
      const inner = parseOr()
      if (peek()?.kind !== 'close') throw new ParseError('missing )')
      at++
      return inner
    }
    const words: string[] = []
    while (peek()?.kind === 'word') words.push((tokens[at++] as { text: string }).text)
    const query = parseQuery(words.join(' '))
    if (!query) throw new ParseError('missing term')
    return { kind: 'term', query }
  }

  const expr = parseOr()
  if (at < tokens.length) throw new ParseError('unexpected token')
  return expr
}

function hasOperator(tokens: Token[]): boolean {
  return tokens.some((t) => t.kind !== 'word')
}

function terms(expr: Expr, negated = false): { query: SearchQuery; negated: boolean }[] {
  switch (expr.kind) {
    case 'term':
      return [{ query: expr.query, negated }]
    case 'not':
      return terms(expr.expr, !negated)
    default:
      return [...terms(expr.left, negated), ...terms(expr.right, negated)]
  }
}

function isRecordValue(value: unknown): boolean {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

// Every record in document order, the root first as the record of last resort.
function recordPaths(root: unknown): PathSegment[][] {
  const out: PathSegment[][] = [[]]
  const visit = (value: unknown, path: PathSegment[]): void => {
    if (Array.isArray(value)) {
      value.forEach((child, i) => {
        if (isRecordValue(child)) out.push([...path, i])
        visit(child, [...path, i])
      })
    } else if (value !== null && typeof value === 'object') {
      for (const [key, child] of Object.entries(value)) visit(child, [...path, key])
    }
  }
  visit(root, [])
  return out
}

// The record paths a match sits in, nearest first, ending with the root.
function enclosingRecords(path: PathSegment[], records: Set<string>): string[] {
  const out: string[] = []
  for (let i = path.length; i > 0; i--) {
    const id = pathId(path.slice(0, i))
    if (records.has(id)) out.push(id)
  }
  out.push(pathId([]))
  return out
}

function isWithin(path: PathSegment[], record: PathSegment[]): boolean {
  return record.length <= path.length && record.every((seg, i) => path[i] === seg)
}

function evaluate(expr: Expr, record: string, holds: Map<SearchQuery, Set<string>>): boolean {
  switch (expr.kind) {
    case 'term':
      return holds.get(expr.query)!.has(record)
    case 'not':
      return !evaluate(expr.expr, record, holds)
    case 'and':
      return evaluate(expr.left, record, holds) && evaluate(expr.right, record, holds)
    case 'or':
      return evaluate(expr.left, record, holds) || evaluate(expr.right, record, holds)
  }
}

export type SearchOutcome = { result: SearchResult } | { error: string } | null

// Runs input over root: a single term as searchJson does, a chained query per record.
export function runSearch(root: unknown, input: string): SearchOutcome {
  if (!input.trim()) return null
  const tokens = tokenize(input)
  if (!hasOperator(tokens)) {
    const query = parseQuery(input)
    return query ? { result: searchJson(root, query) } : null
  }
  let expr: Expr
  try {
    expr = parse(tokens)
  } catch {
    return { error: 'Invalid query' }
  }

  const allRecords = recordPaths(root)
  const recordIds = new Set(allRecords.slice(1).map(pathId))
  const leaves = terms(expr)
  const matchesByTerm = new Map<SearchQuery, SearchMatch[]>()
  const holds = new Map<SearchQuery, Set<string>>()
  const own = new Set<string>()
  for (const { query } of leaves) {
    const matches = termMatches(root, query)
    matchesByTerm.set(query, matches)
    const inside = new Set<string>()
    for (const match of matches) {
      const enclosing = enclosingRecords(match.path, recordIds)
      own.add(enclosing[0])
      for (const id of enclosing) inside.add(id)
    }
    holds.set(query, inside)
  }

  // A record with a match of its own, and every other element of the same array, so NOT can
  // find the elements that lack a match.
  const ownParents = new Set(
    allRecords.filter((p) => p.length > 0 && own.has(pathId(p))).map((p) => pathId(p.slice(0, -1)))
  )
  const candidates = allRecords.filter((p) => {
    const id = pathId(p)
    return own.has(id) || (p.length > 0 && ownParents.has(pathId(p.slice(0, -1))))
  })
  const qualifying = candidates.filter((p) => evaluate(expr, pathId(p), holds))

  const records = new Set(qualifying.map(pathId))
  const ancestors = new Set<string>()
  const byPath = new Map<string, SearchMatch>()
  for (const record of qualifying) addAncestors(ancestors, record)
  for (const { query, negated } of leaves) {
    if (negated) continue
    for (const match of matchesByTerm.get(query)!) {
      if (!qualifying.some((record) => isWithin(match.path, record))) continue
      byPath.set(pathId(match.path), match)
      addAncestors(ancestors, match.path)
    }
  }
  const matches = [...byPath.values()].sort((a, b) => compareDocumentOrder(root, a.path, b.path))
  return {
    result: { matches, byPath, steps: qualifying, unit: 'record', records, ancestors }
  }
}

// Orders two paths as they appear in the document: by the position of the first differing segment.
function compareDocumentOrder(root: unknown, a: PathSegment[], b: PathSegment[]): number {
  let value: unknown = root
  for (let i = 0; i < Math.min(a.length, b.length); i++) {
    if (a[i] !== b[i]) {
      if (Array.isArray(value)) return (a[i] as number) - (b[i] as number)
      const keys = Object.keys(value as Record<string, unknown>)
      return keys.indexOf(String(a[i])) - keys.indexOf(String(b[i]))
    }
    value = (value as Record<string, unknown>)[a[i] as string]
  }
  return a.length - b.length
}
