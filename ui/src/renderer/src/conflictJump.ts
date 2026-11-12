// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { InjectionKey, Ref } from 'vue'
import type { ConflictMessageAnchor } from '../../preload/types'

// The conflict tab shows one file at a time, so a jump from the chat sets the file first and the
// file's merge screen then scrolls to the lines. nonce makes a second jump to the same lines count.
export interface ConflictReveal {
  anchor: ConflictMessageAnchor
  nonce: number
}

export type ConflictJump = (anchor: ConflictMessageAnchor) => void

export const conflictJumpKey: InjectionKey<ConflictJump> = Symbol('conflictJump')
export const conflictRevealKey: InjectionKey<Ref<ConflictReveal | null>> = Symbol('conflictReveal')
