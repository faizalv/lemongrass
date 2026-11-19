// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

export interface StartPlan {
  start: string[]
  // Held tab ids by the tab id of the thinker they wait for.
  held: Map<string, string[]>
}

// Claude caches per model and per config directory, and Codex never shares Claude's cache.
// Antigravity gets its orientation with its first prompt, so it has no system prompt to share.
export function thinkerCacheKey(vendor: string, account?: string, model?: string): string | null {
  if (vendor === 'agy') return null
  return [vendor, vendor === 'claude' ? (account ?? '') : '', model ?? ''].join('\n')
}

// A key in warmKeys already has a started thinker in the project, so its thinkers start at once.
export function planThinkerStart(
  members: { tabId: string; key: string | null }[],
  warmKeys: Set<string>
): StartPlan {
  const plan: StartPlan = { start: [], held: new Map() }
  const firstOfKey = new Map<string, string>()
  for (const { tabId, key } of members) {
    if (key === null || warmKeys.has(key)) {
      plan.start.push(tabId)
      continue
    }
    const first = firstOfKey.get(key)
    if (first === undefined) {
      firstOfKey.set(key, tabId)
      plan.start.push(tabId)
      continue
    }
    const waiting = plan.held.get(first) ?? []
    waiting.push(tabId)
    plan.held.set(first, waiting)
  }
  return plan
}

export interface HoldClock {
  set(fn: () => void, ms: number): unknown
  clear(handle: unknown): void
}

// Which held tabs wait on which first thinker, and which thinkers have started, so a later spawn
// of a key whose thinker already started in the project skips the wait.
export class ThinkerHolds {
  private holds = new Map<string, { waiting: string[]; handle: unknown }>()
  private held = new Set<string>()
  private keys = new Map<string, { projectPath: string; key: string | null }>()
  private started = new Set<string>()

  private readonly startHeld: (tabId: string) => void
  private readonly clock: HoldClock
  private readonly timeoutMs: number

  // Plain fields, since the tests run this file under node's type stripping, which has no parameter properties.
  constructor(startHeld: (tabId: string) => void, clock: HoldClock, timeoutMs: number) {
    this.startHeld = startHeld
    this.clock = clock
    this.timeoutMs = timeoutMs
  }

  plan(projectPath: string, members: { tabId: string; key: string | null }[]): StartPlan {
    for (const { tabId, key } of members) this.keys.set(tabId, { projectPath, key })
    const plan = planThinkerStart(members, this.warmKeys(projectPath))
    for (const [first, waiting] of plan.held) {
      for (const tabId of waiting) this.held.add(tabId)
      const handle = this.clock.set(() => this.release(first), this.timeoutMs)
      this.holds.set(first, { waiting, handle })
    }
    return plan
  }

  isHeld(tabId: string): boolean {
    return this.held.has(tabId)
  }

  noteStarted(tabId: string): void {
    if (this.keys.has(tabId)) this.started.add(tabId)
    this.release(tabId)
  }

  // A first thinker that is gone releases the tabs waiting on it. Reports whether the tab itself was held.
  noteGone(tabId: string): boolean {
    this.started.delete(tabId)
    this.keys.delete(tabId)
    this.release(tabId)
    return this.held.delete(tabId)
  }

  private release(first: string): void {
    const hold = this.holds.get(first)
    if (!hold) return
    this.holds.delete(first)
    this.clock.clear(hold.handle)
    for (const tabId of hold.waiting) if (this.held.delete(tabId)) this.startHeld(tabId)
  }

  private warmKeys(projectPath: string): Set<string> {
    const keys = new Set<string>()
    for (const tabId of this.started) {
      const known = this.keys.get(tabId)
      if (known?.projectPath === projectPath && known.key !== null) keys.add(known.key)
    }
    return keys
  }
}
