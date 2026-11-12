// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { WorkgroupThreadMessage } from '../../preload/types'

// What the git panel says about one conflicted file once the model has worked on it.
export type ConflictFileStatus =
  { kind: 'proposed'; notes: number } | { kind: 'markers'; blocks: number; questions: number }

// Adds newly read messages to the ones already held, by id, oldest first, so a poll that reads
// only the newest page never drops what was paged in before.
export function mergeMessages(
  held: WorkgroupThreadMessage[],
  incoming: WorkgroupThreadMessage[]
): WorkgroupThreadMessage[] {
  const byId = new Map(held.map((message) => [message.id, message]))
  for (const message of incoming) byId.set(message.id, message)
  return [...byId.values()].sort((a, b) => a.id - b.id)
}

// The model's latest submission. A later submit after a follow-up replaces every earlier one.
export function latestSubmit(messages: WorkgroupThreadMessage[]): WorkgroupThreadMessage | null {
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i].kind === 'submit') return messages[i]
  }
  return null
}

// The model's notes on one file, oldest first.
export function notesFor(
  messages: WorkgroupThreadMessage[],
  path: string
): WorkgroupThreadMessage[] {
  return messages.filter((message) => message.kind === 'note' && message.anchor?.path === path)
}

// A question is the model's note on a marker block it left open: `lgrass git conflict note` reads
// the quote from the working file, so it starts with the block's opening marker.
export function isQuestionNote(message: WorkgroupThreadMessage): boolean {
  if (message.kind !== 'note' || !message.anchor) return false
  return /^<{7}(?: |\r?$)/.test(message.anchor.quote.split('\n')[0])
}

// The state of a file the model worked on, as the git panel and the resolver's file picker word it.
export function fileStatusText(status: ConflictFileStatus): string {
  if (status.kind === 'markers') {
    if (status.questions === 0) return 'markers left'
    return status.questions === 1 ? '1 needs your call' : `${status.questions} need your call`
  }
  if (status.notes === 0) return 'proposed'
  return status.notes === 1 ? 'proposed, 1 note' : `proposed, ${status.notes} notes`
}

// A file the latest submission lists is proposed, unless it still holds marker blocks, which the
// model left for the human. A file it does not list has nothing from the model yet.
export function fileStatus(
  messages: WorkgroupThreadMessage[],
  path: string,
  conflictBlocks: number | null | undefined
): ConflictFileStatus | null {
  const submit = latestSubmit(messages)
  if (!submit || !submit.files.includes(path)) return null
  if (conflictBlocks) {
    const asked = notesFor(messages, path).filter(isQuestionNote).length
    return { kind: 'markers', blocks: conflictBlocks, questions: Math.min(asked, conflictBlocks) }
  }
  return { kind: 'proposed', notes: notesFor(messages, path).length }
}

const MARKER = /^(<{7}|={7}|>{7}|\|{7})( |\r?$)/
const CHIP_QUOTE = 48

// A chip names the file and the first line of code it quotes, never a line number, since line
// numbers move as the file is edited and the chip opens the lines wherever they are now. A marker
// line says nothing about the code, so the first line that is not one is shown.
export function chipText(anchor: { path: string; quote: string }): string {
  const name = anchor.path.split('/').pop() ?? anchor.path
  const line = anchor.quote
    .split('\n')
    .map((l) => l.trim())
    .find((l) => l && !MARKER.test(l))
  if (!line) return name
  const short = line.length > CHIP_QUOTE ? `${line.slice(0, CHIP_QUOTE - 1)}…` : line
  return `${name} · ${short}`
}

// The chat names an author once per run of messages from the same author; a submit, shown as an
// event row, ends the run.
export function startsRun(messages: WorkgroupThreadMessage[], index: number): boolean {
  const message = messages[index]
  const previous = messages[index - 1]
  if (!previous || previous.kind === 'submit') return true
  return previous.author !== message.author || previous.tabId !== message.tabId
}

// The first line of a message, for the small attachment a reply shows above its own text.
export function firstLine(body: string, max = 80): string {
  const line =
    body
      .split('\n')
      .find((l) => l.trim())
      ?.trim() ?? ''
  return line.length > max ? `${line.slice(0, max - 1)}…` : line
}
