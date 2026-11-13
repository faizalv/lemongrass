// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type {
  ConflictVerdict,
  ConflictVerdictKind,
  GitConflictChunk,
  GitConflictFile,
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

export interface BlockCard {
  // Stable while the file's blocks stay grouped the same way.
  key: string
  path: string
  // git's block numbers this card covers, more than one when the written text spans several.
  blocks: number[]
  sides: ConflictChunk
  hold: BlockHold
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
  messages: WorkgroupThreadMessage[]
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
    const report = latest(blocks.map((n) => reports.get(n)))
    const verdict = latest(blocks.map((n) => verdicts.get(n)))
    cards.push({
      key: `${file.path}#${blocks[0]}`,
      path: file.path,
      blocks,
      sides: chunk,
      hold,
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
