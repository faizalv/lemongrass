// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

// Up to two initials for a name: the first letters of its first two words, split on space,
// hyphen, underscore, dot and camelCase, or the first two letters of a single word.
export function monogram(name: string): string {
  const words = name
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .split(/[\s\-_.]+/)
    .filter(Boolean)
  if (words.length === 0) return '?'
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase()
  return (words[0][0] + words[1][0]).toUpperCase()
}
