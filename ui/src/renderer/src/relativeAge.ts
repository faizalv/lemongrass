// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

const MINUTE = 60 * 1000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

// How long ago at was, in words, as of now. A time in the future, from a skewed clock, reads as
// just now.
export function formatAge(at: string, now: number): string {
  const time = new Date(at).getTime()
  const elapsed = now - time
  if (elapsed < MINUTE) return 'a moment ago'
  if (elapsed < HOUR) {
    const minutes = Math.floor(elapsed / MINUTE)
    return minutes === 1 ? 'a minute ago' : `${minutes} minutes ago`
  }
  if (elapsed < DAY) {
    const hours = Math.floor(elapsed / HOUR)
    return hours === 1 ? 'an hour ago' : `${hours} hours ago`
  }
  if (elapsed < 2 * DAY) return 'yesterday'
  if (elapsed < 7 * DAY) return `${Math.floor(elapsed / DAY)} days ago`
  return new Date(time).toLocaleDateString()
}
