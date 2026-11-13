// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import { gitStateOf, stageConflict } from './git'
import { buildProposal } from './conflictModel'
import { conflictThreadOf, handConflictTo, refreshConflictMessages } from './conflictThread'
import { saveConflictWork } from './conflictSave'
import {
  blockDigest,
  fileSettled,
  verdictBatch,
  type BlockCard,
  type Draft,
  type FileDecisions,
  type ManualResolved
} from './conflictDecisions'
import type { ProjectRef } from './workspace'
import type { ConflictVerdictBatch } from '../../preload/types'

// The human's side of Decisions, per project and in memory only: drafts not sent yet, the blocks
// resolved by hand in the three panes, and which view the conflict tab shows. A draft belongs to
// the conflict thread it was made in and is dropped when another thread opens.

export type ConflictMode = 'decisions' | 'manual'

// A block the three panes should land on, once, when the tab switches to Manual for it.
export interface ConflictFocus {
  path: string
  block: number
  nonce: number
}

interface ProjectVerdicts {
  // By conflict thread id, so a draft from an earlier thread never applies to a new one.
  drafts: Record<string, Record<string, Draft>>
  // By path, then block number, the digest of the lines the human resolved it to.
  manual: Record<string, ManualResolved>
  mode: ConflictMode | null
  focus: ConflictFocus | null
}

const projects = reactive<Record<string, ProjectVerdicts>>({})
let nonce = 0

const EMPTY: ProjectVerdicts = { drafts: {}, manual: {}, mode: null, focus: null }

// For reads, which a render or a computed may run: never creates state.
function readOf(projectId: string): ProjectVerdicts {
  return projects[projectId] ?? EMPTY
}

// For writes.
function stateOf(projectId: string): ProjectVerdicts {
  if (!projects[projectId])
    projects[projectId] = { drafts: {}, manual: {}, mode: null, focus: null }
  return projects[projectId]
}

function threadOf(projectId: string): string {
  return conflictThreadOf(projectId).status?.threadId ?? ''
}

function draftsFor(projectId: string): Record<string, Draft> {
  const state = stateOf(projectId)
  const thread = threadOf(projectId)
  // Drafts of earlier threads go once a new one is written to.
  for (const id of Object.keys(state.drafts)) if (id !== thread) delete state.drafts[id]
  return (state.drafts[thread] ??= {})
}

// A model holds the conflict while the project's conflict thread is open with an assignee.
export function modelHolds(projectId: string): boolean {
  const thread = conflictThreadOf(projectId)
  return !!thread.status?.assignee && !thread.closed
}

// Decisions while a model holds the conflict, unless the human switched to Manual; Manual always
// without a model.
export function conflictMode(projectId: string): ConflictMode {
  if (!modelHolds(projectId)) return 'manual'
  return readOf(projectId).mode ?? 'decisions'
}

export function setConflictMode(projectId: string, mode: ConflictMode): void {
  stateOf(projectId).mode = mode
}

const NO_DRAFTS: Record<string, Draft> = {}

export function draftsOf(projectId: string): Record<string, Draft> {
  return readOf(projectId).drafts[threadOf(projectId)] ?? NO_DRAFTS
}

export function setDraft(projectId: string, key: string, draft: Draft | null): void {
  const drafts = draftsFor(projectId)
  if (draft) drafts[key] = draft
  else delete drafts[key]
}

export function manualOf(projectId: string, path: string): ManualResolved | undefined {
  return readOf(projectId).manual[path]
}

// Called by the three panes on every change, with the blocks the human resolved there.
export function publishManual(projectId: string, path: string, resolved: ManualResolved): void {
  const state = stateOf(projectId)
  const held = state.manual[path]
  if (held && held.size === resolved.size && [...resolved].every(([n, d]) => held.get(n) === d)) {
    return
  }
  if (resolved.size === 0) delete state.manual[path]
  else state.manual[path] = resolved
}

export function focusOf(projectId: string): ConflictFocus | null {
  return readOf(projectId).focus
}

// The three panes land on a block once they show its file.
export function focusBlock(projectId: string, path: string, block: number): void {
  stateOf(projectId).focus = { path, block, nonce: ++nonce }
}

// "Resolve myself": the card's file opens in Manual on its first block, with the block reopened.
export function resolveByHand(projectId: string, card: BlockCard): void {
  const state = stateOf(projectId)
  draftsFor(projectId)[card.key] = { kind: 'manual' }
  state.focus = { path: card.path, block: card.blocks[0], nonce: ++nonce }
  state.mode = 'manual'
}

// Blocks marked to resolve by hand in one file, which the three panes load open on their base.
export function reopenedBlocks(projectId: string, cards: BlockCard[]): Set<number> {
  const drafts = draftsOf(projectId)
  return new Set(cards.flatMap((card) => (drafts[card.key]?.kind === 'manual' ? card.blocks : [])))
}

// What one file would send on Apply, and whether that settles it.
export function pendingFor(
  projectId: string,
  file: FileDecisions
): { verdicts: ConflictVerdictBatch['verdicts']; settled: boolean } {
  if (!file.cards) return { verdicts: [], settled: false }
  const drafts = draftsOf(projectId)
  const manual = manualOf(projectId, file.path)
  return {
    verdicts: verdictBatch(file.cards, drafts, manual),
    settled: file.cards.length > 0 && fileSettled(file.cards, drafts, manual)
  }
}

// Apply on one file: the hand-resolved blocks go to disk first, then the verdicts to the model
// together, and a file whose every block is settled is staged as it stands. Verdicts on a settled
// file wake nobody, since the model has nothing left to do in it. Returns the refusal, or null.
export async function applyFile(project: ProjectRef, file: FileDecisions): Promise<string | null> {
  if (!file.cards) return 'This file has no blocks to decide.'
  const { verdicts, settled } = pendingFor(project.id, file)
  if (verdicts.length === 0 && !settled) return 'Nothing to send for this file yet.'
  try {
    if (manualOf(project.id, file.path)?.size) {
      const saved = await saveConflictWork(project, file.path)
      if (!saved.ok) return saved.error
    }
    if (verdicts.length > 0) {
      const sent = await window.api.conflictAgent.verdicts(
        project.path,
        { path: file.path, verdicts },
        settled
      )
      if (!sent.ok) return sent.reason
    }
    const drafts = draftsFor(project.id)
    for (const card of file.cards) delete drafts[card.key]
    await refreshConflictMessages(project)
    if (settled) {
      const refused = await stageConflict(project, file.path)
      if (refused) return refused
      delete stateOf(project.id).manual[file.path]
    }
    return null
  } catch (err) {
    return err instanceof Error ? err.message : String(err)
  }
}

// Every block already written in the working files when the conflict is first handed over, one
// quiet batch per file, so the model keeps them and Decisions shows them as done: whatever a file
// holds in place of a block before any model touched it is the human's own work.
export async function writtenBeforeHandOver(project: ProjectRef): Promise<ConflictVerdictBatch[]> {
  // Handing an open conflict to another tab keeps its thread, where every block already stands.
  if (conflictThreadOf(project.id).status) return []
  const paths = (gitStateOf(project.id).status?.files ?? [])
    .filter((file) => file.group === 'conflicted')
    .map((file) => file.path)
  const batches: ConflictVerdictBatch[] = []
  for (const path of paths) {
    const file = await window.api.git.conflict(project.path, path)
    if (file.status !== 'ok' || !file.chunks || file.working === null) continue
    const mapped = buildProposal(file.chunks, file.working, file.workingChunks)
    if (!mapped) continue
    const verdicts = Object.entries(mapped.proposal.lines).flatMap(([index, lines]) =>
      (mapped.proposal.blocks[Number(index)] ?? []).map((block) => ({
        block,
        verdict: 'resolved' as const,
        digest: blockDigest(lines)
      }))
    )
    if (verdicts.length > 0) batches.push({ path, verdicts })
  }
  return batches
}

// What a block's ask says when the human wrote nothing: the ask itself is the message.
const ASK_WITHOUT_WORDS = 'Look at this block and tell me what you would do.'

function askedBatch(path: string, blocks: number[], text: string): ConflictVerdictBatch {
  const words = text.trim() || ASK_WITHOUT_WORDS
  return { path, verdicts: blocks.map((block) => ({ block, verdict: 'asked', text: words })) }
}

// Asks a model about blocks from the three panes. The human's partial work goes to disk first, so
// the model reads what the human sees. With tabId the conflict is handed to that tab in the same
// step, and the question travels with the hand-over so the model wakes once; without it a model
// already holds the conflict and the question is posted to its thread. Returns the refusal, or
// null.
export async function askAboutBlocks(
  project: ProjectRef,
  path: string,
  blocks: number[],
  text: string,
  tabId: string | null
): Promise<string | null> {
  if (blocks.length === 0) return 'Put the cursor in a conflict block first.'
  const question = askedBatch(path, blocks, text)
  try {
    if (tabId) {
      return await handConflictTo(project, tabId, () => writtenBeforeHandOver(project), question)
    }
    const saved = await saveConflictWork(project)
    if (!saved.ok) return saved.error
    const sent = await window.api.conflictAgent.verdicts(project.path, question, false)
    if (!sent.ok) return sent.reason
    await refreshConflictMessages(project)
    return null
  } catch (err) {
    return err instanceof Error ? err.message : String(err)
  }
}
