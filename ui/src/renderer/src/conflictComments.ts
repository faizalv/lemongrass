// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type {
  ConflictCommentPane,
  ConflictHumanMessage,
  ConflictMessageAnchor,
  WorkgroupThreadMessage
} from '../../preload/types'

// Comments on lines of a conflicted file. A comment is a draft until the human sends it, and then
// a human message in the conflict thread with the agent's replies under it. Pure, with no local
// value imports, so node's test runner loads it.

// A comment written and saved on a card, waiting for Send. replyTo is the thread message it answers
// when it is the human's reply to a comment already sent. startLine and endLine are lines of the
// file the pane showed (a stage file for a side, the working file for the result), and the middle
// pane's own lines for the middle; quote is the text they held.
export interface CommentDraft {
  id: string
  path: string
  // git's block number the comment is scoped to, 0 for a file.
  block: number
  pane: ConflictCommentPane
  startLine: number
  endLine: number
  quote: string
  comment: string
  replyTo?: number
}

export interface CommentReply {
  id: number
  author: 'tab' | 'human'
  tabId: string
  label: string
  body: string
}

// A comment already in the thread, with every message that answers it.
export interface SentComment {
  id: number
  anchor: ConflictMessageAnchor & { pane: ConflictCommentPane; block: number }
  body: string
  replies: CommentReply[]
}

function isPane(value: unknown): value is ConflictCommentPane {
  return value === 'left' || value === 'right' || value === 'result' || value === 'middle'
}

// The human's line comments in a thread, those that name a pane and pick lines, each with its
// replies oldest first. A message about a block or a file with no line picked is chat, not a card.
export function sentComments(messages: WorkgroupThreadMessage[]): SentComment[] {
  const comments: SentComment[] = []
  const byId = new Map<number, SentComment>()
  for (const message of messages) {
    const anchor = message.anchor
    if (message.author !== 'human' || message.replyTo !== null || !anchor) continue
    if (!isPane(anchor.pane) || anchor.startLine < 1) continue
    const comment: SentComment = {
      id: message.id,
      anchor: { ...anchor, pane: anchor.pane, block: anchor.block ?? 0 },
      body: message.body,
      replies: []
    }
    comments.push(comment)
    byId.set(message.id, comment)
  }
  for (const message of messages) {
    if (message.replyTo === null) continue
    byId.get(message.replyTo)?.replies.push({
      id: message.id,
      author: message.author,
      tabId: message.tabId,
      label: message.label,
      body: message.body
    })
  }
  return comments
}

// The block a line belongs to in a pane that shows a whole file: the one whose lines hold it, or
// else the nearest by distance, since a comment on a clean line is about the block beside it. 0 when
// the file has no block.
export function blockAtLine(
  blocks: { block: number; start: number; end: number }[],
  line: number
): number {
  let best = 0
  let bestDistance = Infinity
  for (const { block, start, end } of blocks) {
    const distance = line < start ? start - line : line > end ? line - end : 0
    if (distance < bestDistance) {
      best = block
      bestDistance = distance
    }
  }
  return best
}

// The queued comments as the messages the thread takes, in the order they were written. A reply
// carries the anchor of the comment it answers.
export function commentMessages(drafts: CommentDraft[]): ConflictHumanMessage[] {
  return drafts.map((draft) => ({
    body: draft.comment,
    anchor: {
      path: draft.path,
      startLine: draft.startLine,
      endLine: draft.endLine,
      quote: draft.quote,
      pane: draft.pane,
      block: draft.block
    },
    ...(draft.replyTo === undefined ? {} : { replyTo: draft.replyTo })
  }))
}
