// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'

// How the chat box stands, per project and in memory only: open or closed, whether it lists the
// whole file instead of the selected block, the text being written, a nonce that moves the cursor
// into the box when something sends the human there, and the newest message the human saw in it.

interface ChatState {
  open: boolean
  wide: boolean
  text: string
  focus: number
  seen: number
}

const chats = reactive<Record<string, ChatState>>({})
const IDLE: ChatState = { open: false, wide: false, text: '', focus: 0, seen: 0 }

export function chatOf(projectId: string): ChatState {
  return chats[projectId] ?? IDLE
}

function stateOf(projectId: string): ChatState {
  return (chats[projectId] ??= { ...IDLE })
}

export function setChatOpen(projectId: string, open: boolean): void {
  stateOf(projectId).open = open
}

export function setChatWide(projectId: string, wide: boolean): void {
  stateOf(projectId).wide = wide
}

export function setChatText(projectId: string, text: string): void {
  stateOf(projectId).text = text
}

// Only ever moves forward, so a late call with an older thread never brings a count back.
export function markChatSeen(projectId: string, id: number): void {
  const state = stateOf(projectId)
  if (id > state.seen) state.seen = id
}

// Opens the box with the cursor in its textarea.
export function focusChat(projectId: string): void {
  const state = stateOf(projectId)
  state.open = true
  state.focus++
}
