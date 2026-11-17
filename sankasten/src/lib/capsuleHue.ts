// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

// The theme hues a capsule's dot can take. Status hues (success, error) are left out, so a dot's
// color never reads as a state.
export const CAPSULE_HUES = [
  'amber',
  'coral',
  'orange',
  'lime',
  'jade',
  'teal',
  'sky',
  'sapphire',
  'indigo',
  'violet',
  'fuchsia',
  'rose'
] as const

export type CapsuleHue = (typeof CAPSULE_HUES)[number]

// FNV-1a over the UTF-16 code units: stable across runs and machines, so one content always gets
// one hue, and cheap enough to run on every render.
export function capsuleHue(content: string): CapsuleHue {
  let hash = 0x811c9dc5
  for (let i = 0; i < content.length; i++) {
    hash ^= content.charCodeAt(i)
    hash = Math.imul(hash, 0x01000193)
  }
  return CAPSULE_HUES[(hash >>> 0) % CAPSULE_HUES.length]
}

export function isCapsuleHue(value: unknown): value is CapsuleHue {
  return typeof value === 'string' && (CAPSULE_HUES as readonly string[]).includes(value)
}

// For an item whose hue is assigned once and kept, such as a project: a hue none or the fewest of
// the others hold, so items look apart until the palette runs out. The content's own hash wins
// among those, then palette order, so the pick is stable for the same inputs.
export function leastUsedHue(content: string, used: readonly CapsuleHue[]): CapsuleHue {
  const counts = new Map<CapsuleHue, number>(CAPSULE_HUES.map((hue) => [hue, 0]))
  for (const hue of used) counts.set(hue, (counts.get(hue) ?? 0) + 1)
  const fewest = Math.min(...counts.values())
  const own = capsuleHue(content)
  if (counts.get(own) === fewest) return own
  return CAPSULE_HUES.find((hue) => counts.get(hue) === fewest) ?? own
}
