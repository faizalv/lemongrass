// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import type { GitRunner } from './gitMove'
import type { CommitIntent, ConflictIntent, SideIntent } from './reviewCompose'
import type { GitConflictOperation } from '../preload/types'

// Like gitConflict.ts, this file has no value imports so node --test can load it from source.

// Bounds what `lgrass git conflict show` prints, since a long-lived branch can carry hundreds of
// commits touching a file and the model reads all of it as context.
export const INTENT_MAX_COMMITS = 20
export const INTENT_MAX_BODY = 1500

const FIELD = '\x1f'
const RECORD = '\x1e'
const FORMAT = `--format=%h${FIELD}%s${FIELD}%b${RECORD}`

const NONE: SideIntent = { commits: [], more: 0, missing: null }

function trimBody(body: string): string {
  const text = body.trim()
  return text.length > INTENT_MAX_BODY ? `${text.slice(0, INTENT_MAX_BODY).trimEnd()} [cut]` : text
}

function parseLog(stdout: string): CommitIntent[] {
  return stdout
    .split(RECORD)
    .map((record) => record.replace(/^\n+/, ''))
    .filter(Boolean)
    .map((record) => {
      const [hash = '', subject = '', body = ''] = record.split(FIELD)
      return { hash, subject: subject.trim(), body: trimBody(body) }
    })
}

async function verify(git: GitRunner, rev: string): Promise<string | null> {
  const result = await git.read(['rev-parse', '-q', '--verify', `${rev}^{commit}`])
  return result.code === 0 ? result.stdout.trim() || null : null
}

async function mergeBase(git: GitRunner, a: string, b: string): Promise<string | null> {
  const result = await git.read(['merge-base', a, b])
  return result.code === 0 ? result.stdout.trim() || null : null
}

// The commits in base..tip that touched any of paths, newest first, with how many more were left out.
async function commitsTouching(
  git: GitRunner,
  base: string,
  tip: string,
  paths: string[]
): Promise<SideIntent> {
  if (paths.length === 0) return NONE
  const range = `${base}..${tip}`
  const log = await git.read(['log', `-n${INTENT_MAX_COMMITS}`, FORMAT, range, '--', ...paths])
  if (log.code !== 0) return { ...NONE, missing: 'git could not read these commits' }
  const commits = parseLog(log.stdout)
  const count = await git.read(['rev-list', '--count', range, '--', ...paths])
  const total = count.code === 0 ? Number(count.stdout.trim()) || commits.length : commits.length
  return { commits, more: Math.max(0, total - commits.length), missing: null }
}

async function oneCommit(git: GitRunner, rev: string): Promise<SideIntent> {
  const log = await git.read(['log', '-1', FORMAT, rev])
  if (log.code !== 0) return { ...NONE, missing: 'git could not read this commit' }
  return { commits: parseLog(log.stdout), more: 0, missing: null }
}

// What each side meant, from the commits that touched the conflicted paths since the sides parted.
// Sides follow the resolver's panes: in a rebase the left is the branch being replayed, and in a
// stash apply the left is the stashed changes.
export async function readIntent(
  git: GitRunner,
  operation: GitConflictOperation,
  paths: string[]
): Promise<ConflictIntent> {
  if (operation === 'merge') {
    const head = await verify(git, 'HEAD')
    const merged = await verify(git, 'MERGE_HEAD')
    const base = head && merged ? await mergeBase(git, head, merged) : null
    if (!head || !merged || !base) {
      const missing = 'git has no merge base for this merge'
      return { left: { ...NONE, missing }, right: { ...NONE, missing } }
    }
    return {
      left: await commitsTouching(git, base, head, paths),
      right: await commitsTouching(git, base, merged, paths)
    }
  }
  if (operation === 'rebase') {
    const head = await verify(git, 'HEAD')
    const replayed = await verify(git, 'REBASE_HEAD')
    const left = replayed
      ? await oneCommit(git, replayed)
      : { ...NONE, missing: 'git did not record the commit being replayed' }
    const base = head && replayed ? await mergeBase(git, replayed, head) : null
    const right =
      head && base
        ? await commitsTouching(git, base, head, paths)
        : { ...NONE, missing: 'git has no merge base for this rebase' }
    return { left, right }
  }
  // git records no trace of which stash a conflicting apply came from, so the newest is the best
  // guess, which is the one an --autostash reapply leaves.
  const stash = await verify(git, 'refs/stash')
  if (!stash) {
    const missing = 'no stash is left to read'
    return { left: { ...NONE, missing }, right: { ...NONE, missing } }
  }
  const head = await verify(git, 'HEAD')
  const stashBase = await verify(git, `${stash}^1`)
  return {
    left: await oneCommit(git, stash),
    right:
      head && stashBase
        ? await commitsTouching(git, stashBase, head, paths)
        : { ...NONE, missing: 'git has no base for this stash' }
  }
}
