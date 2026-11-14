// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { ConflictHumanMessage, WorkgroupThreadMessage } from '../../preload/types'

// The chat box over a conflict: which thread messages it lists, how a message is scoped by where
// it was sent, and what the box says it is asking about. Pure, with no local value imports, so
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

// Whether a message belongs to a scope. A message sent with no anchor is general and always
// belongs. Otherwise it must be about the same file, and for a block scope about that block or the
// file as a whole, which concerns every block in it.
export function inScope(message: WorkgroupThreadMessage, scope: ChatScope): boolean {
  const anchor = message.anchor
  if (!anchor) return true
  if (anchor.path !== scope.path) return false
  if (scope.block === null) return true
  const block = anchor.block ?? 0
  return block === 0 || block === scope.block
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
