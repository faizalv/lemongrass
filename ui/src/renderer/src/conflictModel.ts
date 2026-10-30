// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { diff } from '@codemirror/merge'
import type { GitConflictChunk, GitLineRange } from '../../preload/types'

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

function stripCr(lines: string[]): string[] {
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
  const next = { ...chunk, [side]: decision, taken }
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
    resolved: true
  }
  return { chunk: next, text: decidedText(conflict, taken) }
}

// The middle starts as git's clean text with each conflict holding its base lines, unresolved.
export function buildMiddle(chunks: GitConflictChunk[]): { text: string; chunks: MiddleChunk[] } {
  let text = ''
  const middle: MiddleChunk[] = []
  chunks.forEach((chunk, index) => {
    const part = lineText(chunk.kind === 'clean' ? chunk.lines : chunk.base)
    if (chunk.kind === 'conflict') {
      middle.push({
        index,
        from: text.length,
        to: text.length + part.length,
        resolved: false,
        left: 'open',
        right: 'open',
        taken: []
      })
    }
    text += part
  })
  return { text, chunks: middle }
}

// The text Apply writes: the middle without the last newline when the file had none, and with
// "\r\n" back when the file used it.
export function applyText(doc: string, finalNewline: boolean, crlf: boolean): string {
  const text = !finalNewline && doc.endsWith('\n') ? doc.slice(0, -1) : doc
  return crlf ? text.replace(/\n/g, '\r\n') : text
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
function encodeLines(a: string[], b: string[]): [string, string] | null {
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
function mapBoundary(
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
// whole marker block, a side pane to that side's lines between the markers.
export function diskSegments(
  chunks: GitConflictChunk[],
  ranges: GitLineRange[],
  pane: ConflictSide | 'middle'
): DiskSegment[] {
  return chunks.map((chunk, index) => {
    const range = ranges[index]
    if (chunk.kind === 'clean') {
      return { pane: range, disk: chunk.disk, exact: sameLength(range, chunk.disk) }
    }
    if (!chunk.disk) return { pane: range, disk: null, exact: false }
    if (pane === 'middle') return { pane: range, disk: chunk.disk, exact: false }
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
