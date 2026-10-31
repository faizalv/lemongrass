// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { diff } from '@codemirror/merge'
import type { GitConflictChunk, GitConflictOperation, GitLineRange } from '../../preload/types'

export type ConflictSide = 'left' | 'right'
export type ConflictTake = ConflictSide | 'both'
export type SideDecision = 'open' | 'taken' | 'rejected'

type ConflictChunk = Extract<GitConflictChunk, { kind: 'conflict' }>

// A conflict chunk's place in the middle document, as character offsets. Every line in the middle
// ends with a newline, so a chunk covers whole lines and an empty chunk has from === to. Each side
// is decided on its own, and taken keeps the order the sides were taken in.
export interface MiddleChunk {
  index: number
  from: number
  to: number
  resolved: boolean
  left: SideDecision
  right: SideDecision
  taken: ConflictSide[]
  // Holds the model's text from a submission and waits for the human: Accept keeps it, a side
  // taken or a hand edit replaces it. Unresolved until then, so Apply waits on it.
  proposed?: boolean
}

// One stretch of a pane mapped to the working file. exact means line n of the pane is line n of the
// disk range; otherwise any line inside maps to the whole disk range.
export interface DiskSegment {
  pane: GitLineRange
  disk: GitLineRange | null
  exact: boolean
}

function lineText(lines: string[]): string {
  return lines.map((line) => `${line}\n`).join('')
}

export function splitLines(text: string): string[] {
  const lines = text.split('\n')
  if (lines[lines.length - 1] === '') lines.pop()
  return lines
}

// CodeMirror joins lines with "\n", so a CRLF file is edited without its "\r" and gets it back on
// Apply.
export function isCrlf(chunks: GitConflictChunk[]): boolean {
  return chunks.some((chunk) =>
    chunk.kind === 'clean'
      ? chunk.lines.some((line) => line.endsWith('\r'))
      : [...chunk.left, ...chunk.right].some((line) => line.endsWith('\r'))
  )
}

export function stripCr(lines: string[]): string[] {
  return lines.map((line) => (line.endsWith('\r') ? line.slice(0, -1) : line))
}

export function stripChunks(chunks: GitConflictChunk[]): GitConflictChunk[] {
  return chunks.map((chunk) =>
    chunk.kind === 'clean'
      ? { ...chunk, lines: stripCr(chunk.lines) }
      : {
          ...chunk,
          left: stripCr(chunk.left),
          base: stripCr(chunk.base),
          right: stripCr(chunk.right)
        }
  )
}

function decidedText(conflict: ConflictChunk, taken: ConflictSide[]): string {
  return lineText(taken.length ? taken.flatMap((side) => conflict[side]) : conflict.base)
}

// Takes or rejects one side of a conflict. The middle holds the taken sides in the order taken, or
// the base once none is, and the conflict is resolved when both sides are decided.
export function decide(
  chunk: MiddleChunk,
  conflict: ConflictChunk,
  side: ConflictSide,
  decision: 'taken' | 'rejected'
): { chunk: MiddleChunk; text: string } {
  const others = chunk.taken.filter((entry) => entry !== side)
  const taken =
    decision === 'rejected' ? others : chunk.taken.includes(side) ? chunk.taken : [...others, side]
  const next = { ...chunk, [side]: decision, taken, ...(chunk.proposed ? { proposed: false } : {}) }
  next.resolved = next.left !== 'open' && next.right !== 'open'
  return { chunk: next, text: decidedText(conflict, next.taken) }
}

// The toolbar's take for a whole file: both sides decided at once, left before right for both.
export function decideAll(
  chunk: MiddleChunk,
  conflict: ConflictChunk,
  take: ConflictTake
): { chunk: MiddleChunk; text: string } {
  const taken: ConflictSide[] = take === 'both' ? ['left', 'right'] : [take]
  const next: MiddleChunk = {
    ...chunk,
    left: taken.includes('left') ? 'taken' : 'rejected',
    right: taken.includes('right') ? 'taken' : 'rejected',
    taken,
    resolved: true,
    ...(chunk.proposed ? { proposed: false } : {})
  }
  return { chunk: next, text: decidedText(conflict, taken) }
}

// The model's submitted work laid over the chunks: the lines it proposed for some conflicts, by
// chunk index, and the lines it changed inside clean chunks, as 0-based inclusive offsets into that
// chunk's lines.
export interface MiddleProposal {
  lines: Record<number, string[]>
  edits: { index: number; start: number; end: number }[]
}

// The middle starts as git's clean text with each conflict holding its base lines, unresolved. With
// a proposal, a proposed conflict holds the model's lines instead, and the model's edits outside
// the conflicts come back as middle lines.
export function buildMiddle(
  chunks: GitConflictChunk[],
  proposal: MiddleProposal | null = null
): { text: string; chunks: MiddleChunk[]; edits: GitLineRange[] } {
  let text = ''
  let line = 1
  const middle: MiddleChunk[] = []
  const edits: GitLineRange[] = []
  chunks.forEach((chunk, index) => {
    const proposed = chunk.kind === 'conflict' ? proposal?.lines[index] : undefined
    const lines = chunk.kind === 'clean' ? chunk.lines : (proposed ?? chunk.base)
    const part = lineText(lines)
    if (chunk.kind === 'conflict') {
      middle.push({
        index,
        from: text.length,
        to: text.length + part.length,
        resolved: false,
        left: 'open',
        right: 'open',
        taken: [],
        ...(proposed ? { proposed: true } : {})
      })
    } else {
      for (const edit of proposal?.edits ?? []) {
        if (edit.index === index) edits.push({ start: line + edit.start, end: line + edit.end })
      }
    }
    text += part
    line += lines.length
  })
  return { text, chunks: middle, edits }
}

// The text Apply writes: the middle without the last newline when the file had none, and with
// "\r\n" back when the file used it.
export function applyText(doc: string, finalNewline: boolean, crlf: boolean): string {
  const text = !finalNewline && doc.endsWith('\n') ? doc.slice(0, -1) : doc
  return crlf ? text.replace(/\n/g, '\r\n') : text
}

function lineCount(text: string): number {
  let count = 0
  for (const char of text) if (char === '\n') count++
  return count
}

// The file the human's partial work is saved as before a review: resolved conflicts as their middle
// text, every other conflict as markers again, so the model sees what is still open. Markers go in
// git's order, stage 2 first, which is the right side in a rebase or a stash, so the marker parser
// maps the sides back the same way. Resolved ranges are lines of the saved file.
export function savedText(
  doc: string,
  middle: MiddleChunk[],
  chunks: GitConflictChunk[],
  refs: { operation: GitConflictOperation; leftRef: string; rightRef: string },
  finalNewline: boolean,
  crlf: boolean
): { text: string; resolved: GitLineRange[] } {
  const first: ConflictSide = refs.operation === 'merge' ? 'left' : 'right'
  const second: ConflictSide = first === 'left' ? 'right' : 'left'
  const refOf = (side: ConflictSide): string => (side === 'left' ? refs.leftRef : refs.rightRef)
  let text = ''
  let at = 0
  const resolved: GitLineRange[] = []
  for (const chunk of [...middle].sort((a, b) => a.from - b.from)) {
    const conflict = chunks[chunk.index]
    if (!conflict || conflict.kind !== 'conflict') continue
    text += doc.slice(at, chunk.from)
    at = chunk.to
    // A proposal still waiting for the human stays as the model wrote it, so saving never puts
    // markers back over the model's work. Only the human's own resolutions are reported.
    if (chunk.proposed && !chunk.resolved) {
      text += doc.slice(chunk.from, chunk.to)
      continue
    }
    if (chunk.resolved) {
      const part = doc.slice(chunk.from, chunk.to)
      const start = lineCount(text) + 1
      resolved.push({ start, end: start + lineCount(part) - 1 })
      text += part
      continue
    }
    text +=
      `<<<<<<< ${refOf(first)}\n` +
      lineText(conflict[first]) +
      '=======\n' +
      lineText(conflict[second]) +
      `>>>>>>> ${refOf(second)}\n`
  }
  text += doc.slice(at)
  return { text: applyText(text, finalNewline, crlf), resolved }
}

// Each chunk's range in a document made of the chunks' lines laid end to end.
function rangesOf(lineCounts: number[]): GitLineRange[] {
  let line = 1
  return lineCounts.map((count) => {
    const range = { start: line, end: line + count - 1 }
    line += count
    return range
  })
}

// Every line becomes one character, so a character diff of the two strings is a line diff. Lines
// beyond what fits in the Basic Multilingual Plane give up.
export function encodeLines(a: string[], b: string[]): [string, string] | null {
  const codes = new Map<string, number>()
  let next = 0x100
  const encode = (lines: string[]): string | null => {
    let out = ''
    for (const line of lines) {
      let code = codes.get(line)
      if (code === undefined) {
        if (next === 0xd800) next = 0xe000
        if (next > 0xffff) return null
        code = next++
        codes.set(line, code)
      }
      out += String.fromCharCode(code)
    }
    return out
  }
  const ea = encode(a)
  const eb = encode(b)
  return ea === null || eb === null ? null : [ea, eb]
}

// Maps a line boundary (0 is before the first line) in a to b through a line diff. A boundary
// inside a changed stretch goes to its start or end, so a mapped range never shrinks past it.
export function mapBoundary(
  changes: readonly { fromA: number; toA: number; fromB: number; toB: number }[],
  point: number,
  edge: 'start' | 'end'
): number {
  let offset = 0
  for (const change of changes) {
    if (point <= change.fromA) return point + offset
    if (point < change.toA) return edge === 'start' ? change.fromB : change.toB
    offset = change.toB - change.toA
  }
  return point + offset
}

// Finds every chunk in one side's file. Laid end to end with that side of every conflict, the
// chunks differ from the file only where the other side changed clean lines, so conflict lines
// match exactly. Returns null when the file is too large to encode.
export function locateChunks(
  chunks: GitConflictChunk[],
  side: ConflictSide,
  file: string
): GitLineRange[] | null {
  const parts = chunks.map((chunk) => (chunk.kind === 'clean' ? chunk.lines : chunk[side]))
  const own = rangesOf(parts.map((lines) => lines.length))
  const encoded = encodeLines(parts.flat(), splitLines(file))
  if (!encoded) return null
  const changes = diff(encoded[0], encoded[1])
  return own.map((range) => {
    const start = mapBoundary(changes, range.start - 1, 'start')
    const end = Math.max(start, mapBoundary(changes, range.end, 'end'))
    return { start: start + 1, end }
  })
}

// Line ranges of every chunk in the middle, clean chunks taking the gaps between the conflicts.
export function middleRanges(
  chunks: GitConflictChunk[],
  conflicts: GitLineRange[],
  lineCount: number
): GitLineRange[] {
  const ranges: GitLineRange[] = []
  let conflict = 0
  let line = 1
  chunks.forEach((chunk, index) => {
    if (chunk.kind === 'conflict') {
      const range = conflicts[conflict++]
      ranges.push(range)
      line = range.end + 1
      return
    }
    const nextConflict = chunks.slice(index + 1).findIndex((c) => c.kind === 'conflict')
    // Only the last clean chunk before a conflict, or before the end, takes the gap.
    const following = chunks[index + 1]
    if (following && following.kind === 'clean') {
      ranges.push({ start: line, end: line - 1 })
      return
    }
    const end = nextConflict === -1 ? lineCount : conflicts[conflict].start - 1
    ranges.push({ start: line, end })
    line = end + 1
  })
  return ranges
}

// Pairs each chunk's range in a pane with where it sits on disk. The middle maps a conflict to its
// whole marker block, a side pane to that side's lines between the markers. A proposed conflict
// has no markers: the middle holds the file's own lines, so it maps line for line while its length
// is unchanged.
export function diskSegments(
  chunks: GitConflictChunk[],
  ranges: GitLineRange[],
  pane: ConflictSide | 'middle',
  proposed: ReadonlySet<number> = new Set()
): DiskSegment[] {
  return chunks.map((chunk, index) => {
    const range = ranges[index]
    if (chunk.kind === 'clean') {
      return { pane: range, disk: chunk.disk, exact: sameLength(range, chunk.disk) }
    }
    if (!chunk.disk) return { pane: range, disk: null, exact: false }
    if (pane === 'middle') {
      return {
        pane: range,
        disk: chunk.disk,
        exact: proposed.has(index) && sameLength(range, chunk.disk)
      }
    }
    const disk = chunk.disk[pane]
    return { pane: range, disk, exact: sameLength(range, disk) }
  })
}

function sameLength(a: GitLineRange, b: GitLineRange | null): boolean {
  return b !== null && a.end - a.start === b.end - b.start
}

function segmentAt(
  segments: DiskSegment[],
  line: number,
  side: 'pane' | 'disk'
): DiskSegment | null {
  const filled = segments.filter((s) => s[side] !== null && s[side]!.end >= s[side]!.start)
  return (
    filled.find((s) => s[side]!.start <= line && line <= s[side]!.end) ??
    // A line past the last chunk, such as the empty line after the final newline.
    (filled.length > 0 && line > filled[filled.length - 1][side]!.end
      ? filled[filled.length - 1]
      : null)
  )
}

function mapLine(
  segments: DiskSegment[],
  line: number,
  edge: 'start' | 'end',
  from: 'pane' | 'disk'
): number | null {
  const to = from === 'pane' ? 'disk' : 'pane'
  const segment = segmentAt(segments, line, from)
  const source = segment?.[from]
  const target = segment?.[to]
  if (!source || !target) return null
  if (segment.exact) return target.start + Math.min(line, source.end) - source.start
  return edge === 'start' ? target.start : target.end
}

// Pane lines to working file lines, 1-based and inclusive, or null when a part is not on disk.
export function toDisk(
  segments: DiskSegment[],
  fromLine: number,
  toLine: number
): { startLine: number; endLine: number } | null {
  const startLine = mapLine(segments, fromLine, 'start', 'pane')
  const endLine = mapLine(segments, toLine, 'end', 'pane')
  if (startLine === null || endLine === null) return null
  return { startLine, endLine: Math.max(startLine, endLine) }
}

export function fromDisk(
  segments: DiskSegment[],
  startLine: number,
  endLine: number
): { fromLine: number; toLine: number } | null {
  const fromLine = mapLine(segments, startLine, 'start', 'disk')
  const toLine = mapLine(segments, endLine, 'end', 'disk')
  if (fromLine === null || toLine === null) return null
  return { fromLine, toLine: Math.max(fromLine, toLine) }
}

// Maps a scroll position, as a line boundary with a fraction, from one pane to another by
// interpolating between the chunk boundaries the two panes share.
export function mapScroll(
  from: GitLineRange[],
  to: GitLineRange[],
  fromTotal: number,
  toTotal: number,
  position: number
): number {
  const points: [number, number][] = [[0, 0]]
  from.forEach((range, index) => {
    const other = to[index]
    if (!other) return
    points.push([range.start - 1, other.start - 1], [range.end, other.end])
  })
  points.push([fromTotal, toTotal])
  for (let i = 1; i < points.length; i++) {
    const [a0, b0] = points[i - 1]
    const [a1, b1] = points[i]
    if (position > a1) continue
    if (a1 <= a0) return b1
    return b0 + ((position - a0) / (a1 - a0)) * (b1 - b0)
  }
  return toTotal
}

// Lines Focus hides: each clean stretch less the context lines next to its conflicts and the lines
// to keep, such as commented ones. A gap shorter than two lines stays, since hiding one line saves
// nothing next to the line saying it is hidden.
export function collapseSpans(
  clean: GitLineRange[],
  keep: GitLineRange[],
  lineCount: number,
  context = 3
): GitLineRange[] {
  const spans: GitLineRange[] = []
  for (const range of clean) {
    if (range.end < range.start) continue
    let pieces = [
      {
        start: range.start === 1 ? 1 : range.start + context,
        end: range.end >= lineCount ? range.end : range.end - context
      }
    ]
    for (const kept of keep) {
      pieces = pieces.flatMap((piece) =>
        kept.end < piece.start || kept.start > piece.end
          ? [piece]
          : [
              { start: piece.start, end: kept.start - 1 },
              { start: kept.end + 1, end: piece.end }
            ]
      )
    }
    spans.push(...pieces.filter((piece) => piece.end - piece.start >= 1))
  }
  return spans
}

// The middle of a resolver opened on the model's submission: the chunks of the model's working
// file, with git's conflicts mapped into it as proposed conflicts.
export interface Proposal {
  chunks: GitConflictChunk[]
  finalNewline: boolean
  proposal: MiddleProposal
}

// One line of the working file as the diff sees it, or a whole marker block the model left.
interface Entry {
  line: number
  // Set on a marker block, which spans these working lines.
  block: ConflictChunk | null
  end: number
}

// A line can never hold a newline, so these never match a line of either file.
const gitSentinel = (index: number): string => `\ngit conflict ${index}`
const blockSentinel = (index: number): string => `\nmarker block ${index}`

// git's conflicts first..last joined into one, the clean lines between them going to every side,
// for when the model's text runs across several of them and they can only be judged together.
function joinConflicts(chunks: GitConflictChunk[], first: number, last: number): ConflictChunk {
  const part = chunks.slice(first, last + 1)
  const side = (key: 'left' | 'base' | 'right'): string[] =>
    part.flatMap((chunk) => (chunk.kind === 'clean' ? chunk.lines : chunk[key]))
  return {
    kind: 'conflict',
    left: side('left'),
    base: side('base'),
    right: side('right'),
    disk: null
  }
}

// Maps git's merge chunks into the model's working file. Laid end to end with every conflict as a
// line that matches nothing, git's chunks differ from the file only where the model wrote: each
// conflict lands on the lines the model put in its place (mapBoundary), and the clean lines the diff
// reports changed are the model's edits outside the conflicts. A conflict that lands on a marker
// block the model left is left to that block, which stays an ordinary open conflict. Returns null
// when the files are too large to encode.
export function buildProposal(
  gitChunks: GitConflictChunk[],
  working: string,
  workingChunks: GitConflictChunk[] | null
): Proposal | null {
  const lines = splitLines(working)
  const segments: GitConflictChunk[] = workingChunks ?? [
    { kind: 'clean', lines, disk: { start: 1, end: lines.length } }
  ]

  const entries: Entry[] = []
  const b: string[] = []
  segments.forEach((segment, index) => {
    if (!segment.disk) return
    if (segment.kind === 'conflict') {
      entries.push({ line: segment.disk.start, block: segment, end: segment.disk.end })
      b.push(blockSentinel(index))
      return
    }
    for (let line = segment.disk.start; line <= segment.disk.end; line++) {
      entries.push({ line, block: null, end: line })
      b.push(stripCr([lines[line - 1] ?? ''])[0])
    }
  })

  const a: string[] = []
  const at: number[] = []
  gitChunks.forEach((chunk, index) => {
    at.push(a.length)
    if (chunk.kind === 'clean') a.push(...stripCr(chunk.lines))
    else a.push(gitSentinel(index))
  })

  const encoded = encodeLines(a, b)
  if (!encoded) return null
  const changes = diff(encoded[0], encoded[1])

  // Each conflict's range of entries [from, to), merged where the model's text joined several.
  const mapped: { first: number; last: number; from: number; to: number }[] = []
  gitChunks.forEach((chunk, index) => {
    if (chunk.kind !== 'conflict') return
    const from = mapBoundary(changes, at[index], 'start')
    const to = Math.max(from, mapBoundary(changes, at[index] + 1, 'end'))
    const previous = mapped[mapped.length - 1]
    if (previous && from < previous.to) {
      previous.last = index
      previous.to = Math.max(previous.to, to)
      return
    }
    mapped.push({ first: index, last: index, from, to })
  })
  const proposed = mapped.filter(
    (range) => !entries.slice(range.from, range.to).some((entry) => entry.block)
  )

  // Entries the diff reports as written by the model, outside every proposed conflict.
  const edited = new Set<number>()
  for (const change of changes) {
    for (let entry = change.fromB; entry < change.toB; entry++) edited.add(entry)
  }
  for (const range of proposed) {
    for (let entry = range.from; entry < range.to; entry++) edited.delete(entry)
  }

  const chunks: GitConflictChunk[] = []
  const proposal: MiddleProposal = { lines: {}, edits: [] }
  let clean: number[] = []
  const flush = (): void => {
    if (clean.length === 0) return
    const index = chunks.length
    const first = entries[clean[0]].line
    chunks.push({
      kind: 'clean',
      lines: clean.map((entry) => lines[entries[entry].line - 1]),
      disk: { start: first, end: entries[clean[clean.length - 1]].line }
    })
    clean.forEach((entry, offset) => {
      if (!edited.has(entry)) return
      const last = proposal.edits[proposal.edits.length - 1]
      if (last && last.index === index && last.end === offset - 1) last.end = offset
      else proposal.edits.push({ index, start: offset, end: offset })
    })
    clean = []
  }
  // The working line an entry index starts at, or one past the file's end.
  const lineAt = (entry: number): number =>
    entry < entries.length ? entries[entry].line : (entries[entries.length - 1]?.end ?? 0) + 1

  let next = 0
  for (let entry = 0; entry <= entries.length; entry++) {
    while (next < proposed.length && proposed[next].from === entry) {
      const range = proposed[next++]
      flush()
      const start = lineAt(range.from)
      const end = start + (range.to - range.from) - 1
      const disk: GitLineRange = { start, end }
      proposal.lines[chunks.length] = entries
        .slice(range.from, range.to)
        .map((item) => stripCr([lines[item.line - 1]])[0])
      chunks.push({
        ...joinConflicts(gitChunks, range.first, range.last),
        disk: { ...disk, left: disk, base: null, right: disk }
      })
      entry = range.to
    }
    if (entry >= entries.length) break
    const item = entries[entry]
    if (item.block) {
      flush()
      chunks.push(item.block)
    } else clean.push(entry)
  }
  flush()

  return { chunks, finalNewline: working.endsWith('\n'), proposal }
}

// The chunk indexes holding a proposal, which the middle maps to disk line for line.
export function proposedIndexes(proposal: MiddleProposal | null): Set<number> {
  return new Set(Object.keys(proposal?.lines ?? {}).map(Number))
}
