// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

// The theme hues a capsule's dot can take. Status hues (success, error) are left out, so a dot's
// color never reads as a state.
export const CAPSULE_HUES = [
  'amber',
  'coral',
  'fuchsia',
  'jade',
  'sapphire',
  'teal',
  'violet'
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
