// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// The last update check, kept on disk so a launch knows a newer release at once and does not ask GitHub again
// within the hour. Pure parts only; updater.ts reads and writes the file.

export interface Release {
  version: string
  assetName: string
  assetUrl: string
  assetSize: number
  sumsUrl: string
}

export interface CheckCache {
  checkedAt: number
  // Whether the check read prereleases too. A cache from the other channel is not used, since its newest
  // release can differ.
  prerelease: boolean
  // The newest release's version, kept even when it has no package for this platform yet.
  version: string
  release: Release | null
}

function isRelease(value: unknown): value is Release {
  const r = value as Release
  return (
    !!r &&
    typeof r.version === 'string' &&
    typeof r.assetName === 'string' &&
    typeof r.assetUrl === 'string' &&
    typeof r.assetSize === 'number' &&
    typeof r.sumsUrl === 'string'
  )
}

// Null for anything unusable: unreadable JSON, a wrong shape, the other channel, or a check time in the future,
// which a clock change can leave and would otherwise hold off the next check.
export function parseCheckCache(text: string, prerelease: boolean, now: number): CheckCache | null {
  let value: unknown
  try {
    value = JSON.parse(text)
  } catch {
    return null
  }
  const c = value as CheckCache
  if (!c || typeof c.checkedAt !== 'number' || !Number.isFinite(c.checkedAt)) return null
  if (typeof c.version !== 'string' || c.prerelease !== prerelease) return null
  if (c.release !== null && !isRelease(c.release)) return null
  if (c.checkedAt > now) return null
  return c
}

// How long until the next check: at once when there is no usable cache or it is older than the interval,
// otherwise when the cached check turns one interval old.
export function nextCheckDelay(checkedAt: number | null, now: number, interval: number): number {
  if (checkedAt === null) return 0
  return Math.min(interval, Math.max(0, checkedAt + interval - now))
}
