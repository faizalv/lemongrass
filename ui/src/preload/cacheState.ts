// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { ContextUsage } from './types'

// Shared by the renderer's warning and main's nudge hold, so both agree on when a tab is cold.

// Claude Code's default cache lifetime, assumed when a session's transcript never showed a write.
// It is only an assumption, so the app never blocks a tab on it.
export const DEFAULT_CACHE_TTL_S = 300

// Below this a cold resume is cheap enough to need no warning: a fresh tab that rereads the task
// costs roughly 40 to 50k, so resuming only clearly costs more above this.
export const COLD_CONTEXT_TOKENS = 80_000

// Milliseconds until the tab's prompt cache expires, counted from its last response; zero or less
// once it has.
export function cacheLeftMs(context: ContextUsage, nowMs: number): number {
  const ttl = context.cache_ttl || DEFAULT_CACHE_TTL_S
  return (context.seen_at + ttl) * 1000 - nowMs
}

// Whether the tab's cache lifetime was read from its transcript rather than assumed.
export function cacheTtlKnown(context: ContextUsage): boolean {
  return !!context.cache_ttl
}

// A tab whose next prompt would rewrite a large context at full price, on a known or an assumed
// lifetime. Main holds nudges on this alone, since holding one costs the human nothing.
export function isColdAndLarge(context: ContextUsage | undefined, nowMs: number): boolean {
  return (
    !!context &&
    context.vendor !== 'codex' &&
    context.used_tokens > COLD_CONTEXT_TOKENS &&
    cacheLeftMs(context, nowMs) <= 0
  )
}

// The close reminder waits until a tab has sat idle for half its cache lifetime, ten minutes at
// most, so a tab the human is still talking to never shows it and a tab left alone shows it while
// closing is still cheap.
const CLOSE_REMINDER_MAX_WAIT_MS = 10 * 60 * 1000

// When the close reminder for the tab's current idle stretch becomes due, in epoch milliseconds.
export function closeReminderDueAt(context: ContextUsage): number {
  const ttlMs = (context.cache_ttl || DEFAULT_CACHE_TTL_S) * 1000
  return context.seen_at * 1000 + Math.min(ttlMs / 2, CLOSE_REMINDER_MAX_WAIT_MS)
}

export function closeReminderDue(context: ContextUsage, nowMs: number): boolean {
  return (
    context.vendor !== 'codex' &&
    nowMs >= closeReminderDueAt(context) &&
    cacheLeftMs(context, nowMs) > 0
  )
}

// An unfinished task whose tab has sat idle this long is one the human may have forgotten.
export const IDLE_TASK_REMINDER_MS = 15 * 60 * 1000
