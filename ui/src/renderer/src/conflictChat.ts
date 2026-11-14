// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { ConflictHumanMessage, WorkgroupThreadMessage } from '../../preload/types'

// The chat box over a conflict: which thread messages it lists, the chip naming where each was
// sent from, whether the agent owes an answer, and what the box says it is asking about. Pure, with no local value imports, so
// node's test runner loads it.

// What the chat is about right now: a file, and one block of it in Decisions. block null is the
// file as a whole, which is the scope of the chat in Manual.
export interface ChatScope {
  path: string
  block: number | null
}

// The messages the chat lists: what the human and the agent wrote to each other. The agent's
// reports and the human's verdicts are Decisions' own and never appear as chat.
export function chatMessages(messages: WorkgroupThreadMessage[]): WorkgroupThreadMessage[] {
  return messages.filter((message) => message.kind === 'message')
}

// The agent's messages in the chat that came after the last one the human saw, for the count on
// the Thread button. seen is the id of the newest message the human saw, 0 for none.
export function unreadCount(messages: WorkgroupThreadMessage[], seen: number): number {
  return chatMessages(messages).filter((message) => message.author === 'tab' && message.id > seen)
    .length
}

// The newest message id in a thread, which the human has seen once the chat is open.
export function newestId(messages: WorkgroupThreadMessage[]): number {
  return messages.reduce((max, message) => Math.max(max, message.id), 0)
}

// The chat waits on the agent while the human wrote something after the agent's last message of any
// kind, since a report or a question also answers what the human said.
export function chatAwaitsAnswer(messages: WorkgroupThreadMessage[]): boolean {
  for (let i = messages.length - 1; i >= 0; i--) {
    const message = messages[i]
    if (message.author === 'tab') return false
    if (message.kind === 'message') return true
  }
  return false
}

function fileName(path: string): string {
  return path.slice(path.lastIndexOf('/') + 1)
}

// The chip a message carries: the file, and the block when it is about one. Null for a general
// message.
export function scopeChip(message: WorkgroupThreadMessage): string | null {
  const anchor = message.anchor
  if (!anchor) return null
  const block = anchor.block ?? 0
  return block > 0 ? `${fileName(anchor.path)}, block ${block}` : fileName(anchor.path)
}

// What the box tells the human it is about to ask, before they send.
export function contextLine(scope: ChatScope): string {
  return scope.block === null
    ? `Asking about ${fileName(scope.path)}`
    : `Asking about ${fileName(scope.path)}, block ${scope.block}`
}

// A line the chat sent, as a message of the thread: anchored to the scope with no lines picked.
export function chatMessageFor(scope: ChatScope, body: string): ConflictHumanMessage {
  return {
    body: body.trim(),
    anchor: {
      path: scope.path,
      startLine: 0,
      endLine: 0,
      quote: '',
      pane: '',
      block: scope.block ?? 0
    }
  }
}
