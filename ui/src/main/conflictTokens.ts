// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { randomUUID } from 'node:crypto'
import type { ConflictHumanMessage, ConflictVerdict } from '../preload/types'

// No value imports outside node builtins, so node --test loads this file straight from source.

// Every write the human makes in a conflict thread is a grant main mints here and `lgrass git
// conflict app --token` redeems through the conflict_claim op. A model never sees a token, so no
// model can post as the human or hand a conflict to a tab.
export type ConflictGrant =
  | { action: 'start'; projectPath: string; scope: string; tabId: string }
  | {
      action: 'post'
      projectPath: string
      scope: string
      messages: ConflictHumanMessage[]
      // A hand-over's messages wake nobody: the typed line does.
      quiet?: boolean
    }
  | { action: 'close'; projectPath: string; scope: string; threadId: string }
  | {
      action: 'verdicts'
      projectPath: string
      scope: string
      path: string
      verdicts: ConflictVerdict[]
      quiet: boolean
    }

export const CONFLICT_TOKEN_TTL_MS = 5 * 60_000

const grants = new Map<string, { grant: ConflictGrant; expires: number }>()

export function mintConflictToken(grant: ConflictGrant, now = Date.now()): string {
  for (const [token, held] of grants) if (held.expires <= now) grants.delete(token)
  const token = randomUUID()
  grants.set(token, { grant, expires: now + CONFLICT_TOKEN_TTL_MS })
  return token
}

// A token is spent by its first claim, even an expired one, so a leaked token can never be retried.
export function claimConflictToken(token: string, now = Date.now()): ConflictGrant | null {
  const held = grants.get(token)
  grants.delete(token)
  return held && held.expires > now ? held.grant : null
}

// The claim reply in the shape lgrass reads, snake_case like every app.sock field.
export function claimReply(grant: ConflictGrant): Record<string, unknown> {
  const base = { action: grant.action, project_path: grant.projectPath, scope: grant.scope }
  if (grant.action === 'start') return { ...base, tab_id: grant.tabId }
  if (grant.action === 'post') {
    return { ...base, messages: grant.messages, quiet: grant.quiet === true }
  }
  if (grant.action === 'verdicts') {
    return { ...base, path: grant.path, verdicts: grant.verdicts, quiet: grant.quiet }
  }
  return { ...base, thread_id: grant.threadId }
}
