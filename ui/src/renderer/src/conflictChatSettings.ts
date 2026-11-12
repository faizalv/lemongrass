// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'

// The conflict chat's size, the same for every conflict tab and kept across restarts.
export const CHAT_MIN_HEIGHT = 120
const DEFAULT_HEIGHT = 240

export const conflictChat = reactive({ height: DEFAULT_HEIGHT, minimized: false })

let loaded = false

export async function loadConflictChat(): Promise<void> {
  if (loaded) return
  loaded = true
  const saved = await window.api.uiState.load()
  if (saved.conflictChatHeight) conflictChat.height = saved.conflictChatHeight
  if (typeof saved.conflictChatMinimized === 'boolean') {
    conflictChat.minimized = saved.conflictChatMinimized
  }
}

export function setConflictChatHeight(height: number): void {
  conflictChat.height = Math.max(CHAT_MIN_HEIGHT, Math.round(height))
  window.api.uiState.save({ conflictChatHeight: conflictChat.height })
}

export function setConflictChatMinimized(minimized: boolean): void {
  conflictChat.minimized = minimized
  window.api.uiState.save({ conflictChatMinimized: minimized })
}
