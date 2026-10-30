// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import type { RunResult } from './gitRun'
import type { GitRunner } from './gitMove'
import type {
  GitActionResult,
  GitConflictChoice,
  GitConflictChunk,
  GitConflictDisk,
  GitConflictFile,
  GitConflictOperation,
  GitConflictStage,
  GitLineRange,
  GitOperationRefs
} from '../preload/types'

// Like gitMove.ts, this file has no value imports outside node builtins so node --test can load it
// straight from source.

// An unusual marker size for our own merge-file run, so a line of file content cannot pass for a
// marker. The working file on disk uses git's default of 7 or the conflict-marker-size attribute.
const OWN_MARKER_SIZE = 32
const DISK_MARKER_MIN = 7
// The bytes git itself looks at when it decides a blob is binary.
const BINARY_PROBE = 8000

type StageNumber = 1 | 2 | 3

interface StageEntry {
  mode: string
  oid: string
}

function errorText(result: RunResult): string {
  return result.stderr.trim() || result.stdout.trim() || 'git failed without a message.'
}

async function gitDir(git: GitRunner): Promise<string | null> {
  const result = await git.read(['rev-parse', '--absolute-git-dir'])
  return result.code === 0 ? result.stdout.trim() || null : null
}

// No merge or rebase stopped with files still in conflict means a stash apply conflicted, which is
// how an --autostash reapply ends too.
export async function conflictOperation(git: GitRunner): Promise<GitConflictOperation> {
  const dir = await gitDir(git)
  if (dir && (existsSync(join(dir, 'rebase-merge')) || existsSync(join(dir, 'rebase-apply')))) {
    return 'rebase'
  }
  return dir && existsSync(join(dir, 'MERGE_HEAD')) ? 'merge' : 'stash'
}

async function stages(
  git: GitRunner,
  path: string
): Promise<Map<StageNumber, StageEntry> | string> {
  const result = await git.read(['ls-files', '-u', '-z', '--', path])
  if (result.code !== 0) return errorText(result)
  const entries = new Map<StageNumber, StageEntry>()
  for (const record of result.stdout.split('\0')) {
    const match = /^(\d+) ([0-9a-f]+) ([123])\t/.exec(record)
    if (match) entries.set(Number(match[3]) as StageNumber, { mode: match[1], oid: match[2] })
  }
  return entries
}

// Which stage each pane reads. git's ours (:2:) is the branch being merged into in a merge, but
// the upstream being replayed onto in a rebase and the current branch in a stash apply.
function sideStages(operation: GitConflictOperation): { left: StageNumber; right: StageNumber } {
  return operation === 'merge' ? { left: 2, right: 3 } : { left: 3, right: 2 }
}

async function readStage(git: GitRunner, entry: StageEntry | undefined): Promise<GitConflictStage> {
  if (!entry) return { state: 'missing' }
  // A submodule's entry is a commit, not a blob.
  if (entry.mode === '160000') return { state: 'binary' }
  const blob = await git.read(['cat-file', 'blob', entry.oid])
  if (blob.code !== 0 || blob.stdout.slice(0, BINARY_PROBE).includes('\0')) {
    return { state: 'binary' }
  }
  return { state: 'text', text: blob.stdout }
}

async function shortName(git: GitRunner, oid: string): Promise<string> {
  const refs = await git.read([
    'for-each-ref',
    '--points-at',
    oid,
    '--format=%(refname:short)',
    'refs/heads',
    'refs/remotes'
  ])
  const first = refs.code === 0 ? refs.stdout.split('\n').find(Boolean) : undefined
  if (first) return first
  const hash = await git.read(['rev-parse', '--short', oid])
  return hash.code === 0 ? hash.stdout.trim() : oid.slice(0, 7)
}

async function currentName(git: GitRunner): Promise<string> {
  const branch = await git.read(['symbolic-ref', '--short', '-q', 'HEAD'])
  if (branch.code === 0 && branch.stdout.trim()) return branch.stdout.trim()
  return shortName(git, 'HEAD')
}

function readGitFile(dir: string, ...names: string[]): string | null {
  for (const name of names) {
    try {
      return readFileSync(join(dir, name), 'utf8').trim()
    } catch {
      // Only one of rebase-merge and rebase-apply exists.
    }
  }
  return null
}

async function sideRefs(
  git: GitRunner,
  operation: GitConflictOperation
): Promise<{ leftRef: string; rightRef: string }> {
  const current = await currentName(git)
  const dir = await gitDir(git)
  if (operation === 'stash' || !dir) return { leftRef: 'stashed changes', rightRef: current }
  if (operation === 'merge') {
    const head = readGitFile(dir, 'MERGE_HEAD')?.split('\n')[0]
    return { leftRef: current, rightRef: head ? await shortName(git, head) : 'MERGE_HEAD' }
  }
  const headName = readGitFile(dir, 'rebase-merge/head-name', 'rebase-apply/head-name')
  const onto = readGitFile(dir, 'rebase-merge/onto', 'rebase-apply/onto')
  return {
    leftRef: headName?.startsWith('refs/heads/') ? headName.slice('refs/heads/'.length) : 'HEAD',
    rightRef: onto ? await shortName(git, onto) : 'upstream'
  }
}

// The names a stopped merge or rebase is described by, from the same refs as the pane headers.
export async function operationRefs(
  git: GitRunner,
  operation: 'merge' | 'rebase'
): Promise<GitOperationRefs> {
  const { leftRef, rightRef } = await sideRefs(git, operation)
  return operation === 'merge'
    ? { from: rightRef, onto: leftRef }
    : { from: leftRef, onto: rightRef }
}

function splitLines(text: string): { lines: string[]; finalNewline: boolean } {
  const finalNewline = text.endsWith('\n')
  const lines = text.split('\n')
  if (finalNewline) lines.pop()
  return { lines, finalNewline }
}

// A marker line is the character repeated size times, then nothing or a space and a label. A CRLF
// file has its markers end in a carriage return.
function markerSize(line: string, char: string, size: number | null): number | null {
  const bare = line.endsWith('\r') ? line.slice(0, -1) : line
  let count = 0
  while (bare[count] === char) count++
  if (count === 0 || (bare.length > count && bare[count] !== ' ')) return null
  if (size === null) return count >= DISK_MARKER_MIN ? count : null
  return count === size ? count : null
}

interface ParsedConflict {
  first: string[]
  base: string[]
  second: string[]
  disk: GitConflictDisk
}

// Splits marker-carrying text into clean runs and conflicts in order, with each one's line range in
// the text. first and second are in the order the markers have them. null when a marker is left
// unclosed.
function parseMarkers(
  lines: string[],
  fixedSize: number | null
): Array<{ kind: 'clean'; lines: string[]; disk: GitLineRange } | ParsedConflict> | null {
  const out: Array<{ kind: 'clean'; lines: string[]; disk: GitLineRange } | ParsedConflict> = []
  let clean: string[] = []
  let cleanStart = 1
  let index = 0
  while (index < lines.length) {
    const size = markerSize(lines[index], '<', fixedSize)
    if (size === null) {
      clean.push(lines[index++])
      continue
    }
    if (clean.length > 0) {
      out.push({ kind: 'clean', lines: clean, disk: { start: cleanStart, end: index } })
    }
    const start = index + 1
    const conflict: ParsedConflict = {
      first: [],
      base: [],
      second: [],
      disk: {
        start,
        end: start,
        left: { start: start + 1, end: start },
        base: null,
        right: { start: start + 1, end: start }
      }
    }
    let section: 'first' | 'base' | 'second' = 'first'
    let closed = false
    let sectionStart = start + 1
    const ranges: Record<'first' | 'base' | 'second', GitLineRange | null> = {
      first: null,
      base: null,
      second: null
    }
    for (index++; index < lines.length; index++) {
      const line = lines[index]
      const lineNo = index + 1
      if (section === 'first' && markerSize(line, '|', size) !== null) {
        ranges.first = { start: sectionStart, end: lineNo - 1 }
        section = 'base'
        sectionStart = lineNo + 1
      } else if (section !== 'second' && markerSize(line, '=', size) !== null) {
        ranges[section] = { start: sectionStart, end: lineNo - 1 }
        section = 'second'
        sectionStart = lineNo + 1
      } else if (section === 'second' && markerSize(line, '>', size) !== null) {
        ranges.second = { start: sectionStart, end: lineNo - 1 }
        conflict.disk.end = lineNo
        closed = true
        index++
        break
      } else {
        conflict[section].push(line)
      }
    }
    if (!closed || !ranges.first || !ranges.second) return null
    conflict.disk.left = ranges.first
    conflict.disk.right = ranges.second
    conflict.disk.base = ranges.base
    out.push(conflict)
    clean = []
    cleanStart = index + 1
  }
  if (clean.length > 0) {
    out.push({ kind: 'clean', lines: clean, disk: { start: cleanStart, end: lines.length } })
  }
  return out
}

// The markers always put git's ours (:2:) first, so the sides swap wherever the left pane reads
// :3:.
function toChunks(
  parsed: NonNullable<ReturnType<typeof parseMarkers>>,
  swap: boolean
): GitConflictChunk[] {
  return parsed.map((part) => {
    if ('kind' in part) return part
    const disk = swap ? { ...part.disk, left: part.disk.right, right: part.disk.left } : part.disk
    return {
      kind: 'conflict',
      left: swap ? part.second : part.first,
      base: part.base,
      right: swap ? part.first : part.second,
      disk
    }
  })
}

// Our merge has no disk positions of its own, so its conflicts take the working file's marker
// blocks in order and its clean runs take the gaps between them. With a different number of
// blocks the file was edited by hand and nothing is placed.
function placeOnDisk(
  chunks: GitConflictChunk[],
  working: GitConflictChunk[] | null,
  workingLineCount: number
): GitConflictChunk[] {
  const blocks = (working ?? []).flatMap((chunk) => (chunk.kind === 'conflict' ? [chunk] : []))
  const conflicts = chunks.filter((chunk) => chunk.kind === 'conflict').length
  if (!working || blocks.length !== conflicts) {
    return chunks.map((chunk) => ({ ...chunk, disk: null }))
  }
  let next = 0
  let cursor = 1
  return chunks.map((chunk) => {
    if (chunk.kind === 'conflict') {
      const disk = blocks[next++].disk
      if (disk) cursor = disk.end + 1
      return { ...chunk, disk }
    }
    const end = next < blocks.length ? (blocks[next].disk?.start ?? cursor) - 1 : workingLineCount
    return { ...chunk, disk: { start: cursor, end } }
  })
}

function readWorking(root: string, path: string): string | null {
  try {
    const text = readFileSync(join(root, path), 'utf8')
    return text.slice(0, BINARY_PROBE).includes('\0') ? null : text
  } catch {
    return null
  }
}

async function mergeStages(
  git: GitRunner,
  entries: Map<StageNumber, StageEntry>,
  sides: { left: StageNumber; right: StageNumber }
): Promise<{ chunks: GitConflictChunk[]; finalNewline: boolean } | string> {
  const oid = (stage: StageNumber): string => entries.get(stage)?.oid ?? ''
  const merged = await git.read([
    'merge-file',
    '-p',
    '--diff3',
    '--object-id',
    `--marker-size=${OWN_MARKER_SIZE}`,
    oid(sides.left),
    oid(1),
    oid(sides.right)
  ])
  // merge-file exits with the number of conflicts, capped at 127, and above that on an error.
  if (merged.code === null || merged.code > 127) return errorText(merged)
  const { lines, finalNewline } = splitLines(merged.stdout)
  const parsed = parseMarkers(lines, OWN_MARKER_SIZE)
  if (!parsed) return 'git merge-file returned unbalanced markers.'
  // Our own run already put the left side first.
  return { chunks: toChunks(parsed, false), finalNewline }
}

export async function readConflict(
  git: GitRunner,
  root: string,
  path: string
): Promise<GitConflictFile> {
  const entries = await stages(git, path)
  if (typeof entries === 'string') return { status: 'error', error: entries }
  if (entries.size === 0) return { status: 'notConflicted' }
  const operation = await conflictOperation(git)
  const sides = sideStages(operation)
  const [base, left, right] = await Promise.all([
    readStage(git, entries.get(1)),
    readStage(git, entries.get(sides.left)),
    readStage(git, entries.get(sides.right))
  ])
  const { leftRef, rightRef } = await sideRefs(git, operation)
  const working = readWorking(root, path)
  const workingSplit = working === null ? null : splitLines(working)
  const workingParsed = workingSplit ? parseMarkers(workingSplit.lines, null) : null
  const workingChunks =
    workingParsed && workingParsed.some((part) => !('kind' in part))
      ? toChunks(workingParsed, sides.left === 3)
      : null

  let chunks: GitConflictChunk[] | null = null
  let finalNewline = workingSplit?.finalNewline ?? true
  if (base.state === 'text' && left.state === 'text' && right.state === 'text') {
    const merged = await mergeStages(git, entries, sides)
    if (typeof merged === 'string') return { status: 'error', error: merged }
    chunks = placeOnDisk(merged.chunks, workingChunks, workingSplit?.lines.length ?? 0)
    finalNewline = merged.finalNewline
  }
  return {
    status: 'ok',
    path,
    operation,
    leftRef,
    rightRef,
    base,
    left,
    right,
    chunks,
    finalNewline,
    working,
    workingChunks
  }
}

const notConflicted = 'That file is no longer in conflict.'

// Only a conflicted path may be written, so this call cannot overwrite any other file.
export async function applyResolution(
  git: GitRunner,
  root: string,
  path: string,
  content: string
): Promise<GitActionResult> {
  const entries = await stages(git, path)
  if (typeof entries === 'string') return { ok: false, error: entries }
  if (entries.size === 0) return { ok: false, error: notConflicted }
  try {
    const target = join(root, path)
    mkdirSync(dirname(target), { recursive: true })
    writeFileSync(target, content)
  } catch (err) {
    return { ok: false, error: `Could not write ${path}: ${(err as Error).message}` }
  }
  const added = await git.write(['add', '--', path])
  return added.code === 0 ? { ok: true } : { ok: false, error: errorText(added) }
}

// Taking a side that deleted the file deletes it.
export async function pickSide(
  git: GitRunner,
  path: string,
  choice: GitConflictChoice
): Promise<GitActionResult> {
  const entries = await stages(git, path)
  if (typeof entries === 'string') return { ok: false, error: entries }
  if (entries.size === 0) return { ok: false, error: notConflicted }
  const sides = sideStages(await conflictOperation(git))
  const stage = choice === 'delete' ? null : sides[choice]
  if (stage === null || !entries.has(stage)) {
    const removed = await git.write(['rm', '--quiet', '--', path])
    return removed.code === 0 ? { ok: true } : { ok: false, error: errorText(removed) }
  }
  const checkedOut = await git.write(['checkout', stage === 2 ? '--ours' : '--theirs', '--', path])
  if (checkedOut.code !== 0) return { ok: false, error: errorText(checkedOut) }
  const added = await git.write(['add', '--', path])
  return added.code === 0 ? { ok: true } : { ok: false, error: errorText(added) }
}
