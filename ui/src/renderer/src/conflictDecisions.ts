// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type {
  ConflictVerdict,
  ConflictVerdictKind,
  GitConflictChunk,
  GitConflictFile,
  GitConflictStage,
  GitLineRange,
  WorkgroupThreadMessage
} from '../../preload/types'
import type { ConflictSide, Proposal } from './conflictModel'

// What the Decisions view shows: every block of every conflicted file, its state read from the
// latest word on it in the conflict thread (the model's decision or question, or the human's
// verdict) checked against what the working file holds now. Pure and with no local value imports,
// so node's test runner loads it; the caller maps the working file with conflictModel's
// buildProposal and passes the result in.

type ConflictChunk = Extract<GitConflictChunk, { kind: 'conflict' }>
type OkFile = Extract<GitConflictFile, { status: 'ok' }>

// What a block holds in the working file: still its markers, or text written in their place,
// with the sides that text equals when it is one side or both in an order.
export type BlockHold =
  { kind: 'markers' } | { kind: 'written'; lines: string[]; sides: ConflictSide[] | null }

// question and take wait for the human; changed is an accepted or resolved block whose text
// changed since; mismatch is a block whose report the file contradicts, or written with no report;
// waiting is with the model after a verdict; open is untouched; accepted and resolved are done.
export type CardState =
  'question' | 'take' | 'changed' | 'mismatch' | 'waiting' | 'open' | 'accepted' | 'resolved'

// markersLeft: the model reported a decision, or the human accepted, yet the markers are there.
// unreported: the block was written with no decision on it, or after a question.
export type Mismatch = 'markersLeft' | 'unreported'

export interface Report {
  id: number
  kind: 'decision' | 'question'
  text: string
  options: string[]
}

export interface SentVerdict {
  id: number
  verdict: ConflictVerdictKind
  text: string
  digest: string
}

// Clean lines next to a block, nearest first away from it: before ends at the block, after starts
// where it ends. A neighbour that is itself a conflict gives none.
export interface BlockContext {
  before: string[]
  after: string[]
  // The line number of the first line a box shows, context included, in the file it was read
  // from: a stage file for a side, the working file for the result. Null when it is not known.
  start: number | null
}

// Where each chunk of git's merge sits in a side's own stage file, from conflictModel's
// locateChunks, which the caller runs since this module takes no local value imports.
export interface Located {
  left: GitLineRange[]
  right: GitLineRange[]
}

// How many clean lines each side of a block the workspace shows: a little around each side, since
// the sides are compared, and more around the result, which is judged in its place.
export const SIDE_CONTEXT_LINES = 3
export const RESULT_CONTEXT_LINES = 10

export interface BlockCard {
  // Stable while the file's blocks stay grouped the same way.
  key: string
  path: string
  // git's block numbers this card covers, more than one when the written text spans several.
  blocks: number[]
  sides: ConflictChunk
  hold: BlockHold
  // The clean lines around the block in each side's own file, and around it in the working file for
  // the result.
  leftContext: BlockContext
  rightContext: BlockContext
  resultContext: BlockContext
  // Of the written lines, empty while the block holds markers.
  digest: string
  state: CardState
  mismatch: Mismatch | null
  report: Report | null
  verdict: SentVerdict | null
}

export interface FileDecisions {
  path: string
  leftRef: string
  rightRef: string
  // null when the file has no line blocks, such as a binary file or one deleted on a side.
  cards: BlockCard[] | null
  // Lines written outside every block, which no block report covers.
  editsOutside: number
}

export function contextAround(
  chunks: GitConflictChunk[],
  index: number,
  count: number
): BlockContext {
  const before = chunks[index - 1]
  const after = chunks[index + 1]
  return {
    before: before?.kind === 'clean' ? before.lines.slice(-count) : [],
    after: after?.kind === 'clean' ? after.lines.slice(0, count) : [],
    start: null
  }
}

function stageLines(stage: GitConflictStage): string[] | null {
  if (stage.state !== 'text') return null
  const lines = stage.text.split('\n')
  if (lines[lines.length - 1] === '') lines.pop()
  return lines
}

// A side's context read from its own stage file, so its line numbers are the file's: the clean
// lines next to the block, never reaching into a neighbouring conflict. Without the stage, or when
// it could not be located, the clean lines of git's merge stand in and carry no line numbers.
function sideContext(
  chunks: GitConflictChunk[],
  first: number,
  last: number,
  ranges: GitLineRange[] | undefined,
  stage: GitConflictStage
): BlockContext {
  const lines = stageLines(stage)
  if (!ranges || !lines || first === -1 || last === -1 || !ranges[first] || !ranges[last]) {
    return {
      before: first === -1 ? [] : contextAround(chunks, first, SIDE_CONTEXT_LINES).before,
      after: last === -1 ? [] : contextAround(chunks, last, SIDE_CONTEXT_LINES).after,
      start: null
    }
  }
  const startAt = ranges[first].start
  const endAt = ranges[last].end
  const previous = chunks[first - 1]
  const next = chunks[last + 1]
  const reach = previous?.kind === 'clean' ? ranges[first - 1].start : startAt
  const limit = next?.kind === 'clean' ? ranges[last + 1].end : endAt
  const before = lines.slice(Math.max(reach - 1, startAt - 1 - SIDE_CONTEXT_LINES), startAt - 1)
  const after = lines.slice(endAt, Math.min(limit, endAt + SIDE_CONTEXT_LINES))
  return { before, after, start: startAt - before.length }
}

// The chunk index of git's nth conflict, counting from 1, which is block n.
function gitConflictIndex(chunks: GitConflictChunk[], block: number): number {
  let seen = 0
  for (let i = 0; i < chunks.length; i++) {
    if (chunks[i].kind === 'conflict' && ++seen === block) return i
  }
  return -1
}

function stripCr(lines: string[]): string[] {
  return lines.map((line) => (line.endsWith('\r') ? line.slice(0, -1) : line))
}

function sameLines(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((line, i) => line === b[i])
}

// The sides whose lines the model's text for a conflict equals: one side, or both in either order.
// Null when it wrote anything else, or nothing.
export function matchSides(conflict: ConflictChunk, lines: string[]): ConflictSide[] | null {
  if (lines.length === 0) return null
  const text = stripCr(lines)
  const left = stripCr(conflict.left)
  const right = stripCr(conflict.right)
  const orders: ConflictSide[][] = [['left'], ['right'], ['left', 'right'], ['right', 'left']]
  return (
    orders.find((order) =>
      sameLines(
        text,
        order.flatMap((side) => (side === 'left' ? left : right))
      )
    ) ?? null
  )
}

// A short fingerprint of a block's lines, carriage returns ignored, which a verdict carries so the
// resolver sees when an accepted or resolved block changed afterwards. FNV-1a, 32 bits, plus the
// length: enough to tell one block's text from its next version, not a security check.
export function blockDigest(lines: string[]): string {
  const text = stripCr(lines).join('\n')
  let hash = 0x811c9dc5
  for (let i = 0; i < text.length; i++) {
    hash ^= text.charCodeAt(i)
    hash = Math.imul(hash, 0x01000193) >>> 0
  }
  return `${text.length.toString(36)}-${hash.toString(36)}`
}

interface BlockReportData {
  path: string
  block: number
  options: string[]
}

// A model's decision or question as the thread holds it, or null when its data is not one.
export function reportOf(message: WorkgroupThreadMessage): BlockReportData | null {
  if (message.author !== 'tab' || (message.kind !== 'decision' && message.kind !== 'question')) {
    return null
  }
  const data = message.data as { path?: unknown; block?: unknown; options?: unknown } | null
  if (typeof data?.path !== 'string' || !Number.isInteger(data.block)) return null
  const options = Array.isArray(data.options)
    ? data.options.filter((o): o is string => typeof o === 'string')
    : []
  return { path: data.path, block: data.block as number, options }
}

const VERDICTS = new Set<string>(['accepted', 'rejected', 'resolved', 'answered', 'asked'])

// The human's verdicts batch as the thread holds it, or null when its data is not one.
export function verdictsOf(
  message: WorkgroupThreadMessage
): { path: string; verdicts: ConflictVerdict[] } | null {
  if (message.author !== 'human' || message.kind !== 'verdicts') return null
  const data = message.data as { path?: unknown; verdicts?: unknown } | null
  if (typeof data?.path !== 'string' || !Array.isArray(data.verdicts)) return null
  const verdicts = (data.verdicts as Partial<ConflictVerdict>[]).filter(
    (v): v is ConflictVerdict => Number.isInteger(v?.block) && VERDICTS.has(String(v?.verdict))
  )
  return { path: data.path, verdicts }
}

// The latest model report and human verdict on each block of one file, by block number.
function wordsOn(
  messages: WorkgroupThreadMessage[],
  path: string
): { reports: Map<number, Report>; verdicts: Map<number, SentVerdict> } {
  const reports = new Map<number, Report>()
  const verdicts = new Map<number, SentVerdict>()
  for (const message of messages) {
    const report = reportOf(message)
    if (report?.path === path) {
      reports.set(report.block, {
        id: message.id,
        kind: message.kind as Report['kind'],
        text: message.body,
        options: report.options
      })
      continue
    }
    const batch = verdictsOf(message)
    if (batch?.path !== path) continue
    for (const v of batch.verdicts) {
      verdicts.set(v.block, {
        id: message.id,
        verdict: v.verdict,
        text: v.text ?? '',
        digest: v.digest ?? ''
      })
    }
  }
  return { reports, verdicts }
}

function latest<T extends { id: number }>(items: (T | undefined)[]): T | null {
  let best: T | null = null
  for (const item of items) if (item && (!best || item.id > best.id)) best = item
  return best
}

function stateOf(
  hold: BlockHold,
  digest: string,
  report: Report | null,
  verdict: SentVerdict | null
): { state: CardState; mismatch: Mismatch | null } {
  const written = hold.kind === 'written'
  if (verdict && (!report || verdict.id > report.id)) {
    if (verdict.verdict === 'accepted' || verdict.verdict === 'resolved') {
      if (!written) return { state: 'mismatch', mismatch: 'markersLeft' }
      if (digest === verdict.digest) return { state: verdict.verdict, mismatch: null }
      return { state: 'changed', mismatch: null }
    }
    return { state: 'waiting', mismatch: null }
  }
  if (report?.kind === 'decision') {
    return written
      ? { state: 'take', mismatch: null }
      : { state: 'mismatch', mismatch: 'markersLeft' }
  }
  if (report?.kind === 'question') {
    return written
      ? { state: 'mismatch', mismatch: 'unreported' }
      : { state: 'question', mismatch: null }
  }
  return written ? { state: 'mismatch', mismatch: 'unreported' } : { state: 'open', mismatch: null }
}

// Every block of one conflicted file and where it stands. mapped is buildProposal on the file:
// the working file mapped onto git's merge of the stages, so a block keeps its number however the
// file was edited. Null when the file has no line blocks or is too large to map.
export function fileDecisions(
  file: OkFile,
  mapped: Proposal | null,
  messages: WorkgroupThreadMessage[],
  located: Located | null = null
): FileDecisions {
  const base = { path: file.path, leftRef: file.leftRef, rightRef: file.rightRef, editsOutside: 0 }
  if (!file.chunks || file.working === null || !mapped) return { ...base, cards: null }
  const { reports, verdicts } = wordsOn(messages, file.path)
  const cards: BlockCard[] = []
  mapped.chunks.forEach((chunk, index) => {
    if (chunk.kind !== 'conflict') return
    const blocks = mapped.proposal.blocks[index] ?? []
    if (blocks.length === 0) return
    const lines = mapped.proposal.lines[index]
    const hold: BlockHold = lines
      ? { kind: 'written', lines, sides: matchSides(chunk, lines) }
      : { kind: 'markers' }
    const digest = lines ? blockDigest(lines) : ''
    const first = gitConflictIndex(file.chunks!, blocks[0])
    const last = gitConflictIndex(file.chunks!, blocks[blocks.length - 1])
    const result = contextAround(mapped.chunks, index, RESULT_CONTEXT_LINES)
    if (chunk.disk) result.start = chunk.disk.start - result.before.length
    const report = latest(blocks.map((n) => reports.get(n)))
    const verdict = latest(blocks.map((n) => verdicts.get(n)))
    cards.push({
      key: `${file.path}#${blocks[0]}`,
      path: file.path,
      blocks,
      sides: chunk,
      hold,
      leftContext: sideContext(file.chunks!, first, last, located?.left, file.left),
      rightContext: sideContext(file.chunks!, first, last, located?.right, file.right),
      resultContext: result,
      digest,
      ...stateOf(hold, digest, report, verdict),
      report,
      verdict
    })
  })
  const editsOutside = mapped.proposal.edits.reduce((sum, e) => sum + e.end - e.start + 1, 0)
  return { ...base, cards, editsOutside }
}

// The human's word on a card not sent yet: accept the take as it stands, reject it with a
// sentence for the model, answer a question, or resolve it by hand in the three panes.
export type Draft =
  | { kind: 'accept'; digest: string }
  | { kind: 'reject'; text: string }
  | { kind: 'answer'; text: string; question: number }
  | { kind: 'manual' }

// Blocks the human resolved in the three panes and not sent yet, by block number, with the digest
// of the lines they now hold.
export type ManualResolved = Map<number, string>

// A draft still fits its card: an accept only on the very text it was given for, a reject on
// anything the model wrote, an answer on the question it answers.
export function draftFits(card: BlockCard, draft: Draft | undefined): boolean {
  if (!draft) return false
  if (draft.kind === 'manual') return true
  if (draft.kind === 'accept') {
    return (card.state === 'take' || card.state === 'changed') && draft.digest === card.digest
  }
  if (draft.kind === 'reject') {
    return (
      (card.state === 'take' || card.state === 'changed' || card.state === 'mismatch') &&
      draft.text.trim() !== ''
    )
  }
  return card.state === 'question' && card.report?.id === draft.question && draft.text.trim() !== ''
}

// The digest of a card the human resolved by hand, when every one of its blocks was.
export function manualDigest(card: BlockCard, manual: ManualResolved | undefined): string | null {
  if (!manual) return null
  const digests = card.blocks.map((n) => manual.get(n))
  if (digests.some((d) => d === undefined)) return null
  return digests[0] ?? null
}

// One file's verdicts to send on Apply, one entry per block a card covers.
export function verdictBatch(
  cards: BlockCard[],
  drafts: Record<string, Draft>,
  manual: ManualResolved | undefined
): ConflictVerdict[] {
  const out: ConflictVerdict[] = []
  for (const card of cards) {
    const resolved = manualDigest(card, manual)
    const draft = drafts[card.key]
    const each = (verdict: Omit<ConflictVerdict, 'block'>): void => {
      for (const block of card.blocks) out.push({ block, ...verdict })
    }
    // A hand resolution already sent and still on disk needs no second verdict.
    const sent = card.state === 'resolved' && card.verdict?.digest === resolved
    if (resolved !== null && sent) continue
    if (resolved !== null) each({ verdict: 'resolved', digest: resolved })
    else if (!draftFits(card, draft) || !draft || draft.kind === 'manual') continue
    else if (draft.kind === 'accept') each({ verdict: 'accepted', digest: draft.digest })
    else if (draft.kind === 'reject') each({ verdict: 'rejected', text: draft.text.trim() })
    else each({ verdict: 'answered', text: draft.text.trim(), question: draft.question })
  }
  return out
}

// A file is ready to stage when every block is done or about to be: accepted or resolved already,
// accepted in a draft, or resolved by hand.
export function fileSettled(
  cards: BlockCard[],
  drafts: Record<string, Draft>,
  manual: ManualResolved | undefined
): boolean {
  return cards.every((card) => {
    if (manualDigest(card, manual) !== null) return true
    if (card.state === 'accepted' || card.state === 'resolved') return true
    const draft = drafts[card.key]
    return draft?.kind === 'accept' && draftFits(card, draft)
  })
}

// The model's latest plain message, shown as the one general line above the decisions.
export function generalLine(messages: WorkgroupThreadMessage[]): WorkgroupThreadMessage | null {
  for (let i = messages.length - 1; i >= 0; i--) {
    const m = messages[i]
    if (m.author === 'tab' && m.kind === 'message') return m
  }
  return null
}

// The model ended its round with submit after the human last wrote.
export function roundDone(messages: WorkgroupThreadMessage[]): boolean {
  let submit = 0
  let human = 0
  for (const m of messages) {
    if (m.kind === 'submit') submit = m.id
    if (m.author === 'human') human = m.id
  }
  return submit > human
}

// What the git panel and the file picker say about a file the model worked on.
export function fileStatusText(cards: BlockCard[] | null): string | null {
  if (!cards || cards.length === 0) return null
  const count = (states: CardState[]): number =>
    cards.filter((card) => states.includes(card.state)).length
  const questions = count(['question'])
  const review = count(['take', 'changed', 'mismatch'])
  const done = count(['accepted', 'resolved'])
  if (questions > 0) return questions === 1 ? '1 question' : `${questions} questions`
  if (review > 0) return review === 1 ? '1 to review' : `${review} to review`
  if (done === cards.length) return 'all settled'
  return null
}

// Where a block sits in the Decisions list. needs waits for the human; queued is decided and goes
// to the model on Send; model is with the model; untouched is a block nobody was asked to work
// on, or one the model's finished round never reported; done is accepted or resolved.
export type ListState = 'needs' | 'queued' | 'model' | 'untouched' | 'done'

// The blocks the hand-over asked about, by path, or null when the conflict was handed over whole.
// The hand-over posts the human's resolved blocks and then the asked ones before anything else is
// said, so only the opening verdict batches of the thread count: a later ask, once the model has
// reported or the human has written, says nothing about how the conflict was handed over.
export function handOverAsked(messages: WorkgroupThreadMessage[]): Map<string, Set<number>> | null {
  const asked = new Map<string, Set<number>>()
  for (const message of messages) {
    const batch = verdictsOf(message)
    if (!batch) break
    const kinds = batch.verdicts.map((v) => v.verdict)
    if (kinds.every((kind) => kind === 'resolved' || kind === 'accepted')) continue
    if (!kinds.every((kind) => kind === 'asked')) break
    const blocks = asked.get(batch.path) ?? new Set<number>()
    for (const v of batch.verdicts) blocks.add(v.block)
    asked.set(batch.path, blocks)
  }
  return asked.size > 0 ? asked : null
}

// Whether the human's word on a card waits for Send: an accept, reject or answer that still fits,
// or a block resolved in the three panes that the thread does not hold yet.
export function isQueued(
  card: BlockCard,
  draft: Draft | undefined,
  manual: ManualResolved | undefined
): boolean {
  const resolved = manualDigest(card, manual)
  if (resolved !== null && !(card.state === 'resolved' && card.verdict?.digest === resolved)) {
    return true
  }
  return !!draft && draft.kind !== 'manual' && draftFits(card, draft)
}

// finished is the model's round being over with nothing from the human since, so a block it never
// reported is not coming. asked is handOverAsked.
export function listStateOf(
  card: BlockCard,
  queued: boolean,
  asked: Map<string, Set<number>> | null,
  finished: boolean
): ListState {
  if (queued) return 'queued'
  switch (card.state) {
    case 'question':
    case 'take':
    case 'changed':
    case 'mismatch':
      return 'needs'
    case 'accepted':
    case 'resolved':
      return 'done'
    case 'waiting':
      return 'model'
    case 'open': {
      if (finished) return 'untouched'
      const blocks = asked?.get(card.path)
      if (asked && !card.blocks.some((n) => blocks?.has(n))) return 'untouched'
      return 'model'
    }
  }
}

// The one or two words on a list row.
export function listLabel(state: ListState, card: BlockCard): string {
  switch (state) {
    case 'queued':
      return 'Queued'
    case 'model':
      return 'With agent'
    case 'untouched':
      return 'Untouched'
    case 'done':
      return 'Done'
    case 'needs':
      break
  }
  if (card.state === 'question') return 'Question'
  if (card.state === 'take') return 'Review'
  if (card.state === 'changed') return 'Changed'
  return 'Check'
}

export type ListCounts = Record<ListState, number>

export function countStates(states: ListState[]): ListCounts {
  const counts: ListCounts = { needs: 0, queued: 0, model: 0, untouched: 0, done: 0 }
  for (const state of states) counts[state]++
  return counts
}

// What the human should do next, in one sentence, from where the blocks stand.
export function nextStep(counts: ListCounts): string {
  const blocks = (n: number): string => `${n} block${n === 1 ? '' : 's'}`
  if (counts.needs > 0) {
    const untouched = counts.untouched > 0 ? ` ${counts.untouched} untouched.` : ''
    return `${blocks(counts.needs)} ${counts.needs === 1 ? 'needs' : 'need'} you.${untouched}`
  }
  if (counts.queued > 0) return `${counts.queued} decided and waiting for Send.`
  if (counts.untouched > 0) {
    return `Nothing needs you. ${counts.untouched} untouched: ask an agent or resolve them yourself.`
  }
  if (counts.model > 0) return 'Nothing needs you while the agent works.'
  return 'Everything is settled.'
}
