// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

// Licenses that may ship alongside Lemongrass's AGPL-3.0-only code. Anything else fails the
// packaged build, so a dependency that relicenses to something incompatible is caught before release.
export const ACCEPTED = new Set([
  'MIT',
  'ISC',
  'BSD-2-Clause',
  'BSD-3-Clause',
  '0BSD',
  'Apache-2.0',
  'OFL-1.1',
  'MPL-2.0',
  'Python-2.0',
  'CC0-1.0',
  'Unlicense'
])

// An SPDX expression passes when an OR branch passes and when every AND operand passes. A
// `WITH` exception only adds permissions, so it is judged by the license it modifies.
export function licenseAccepted(expression: string): boolean {
  const tokens = expression.replace(/[()]/g, ' $& ').trim().split(/\s+/).filter(Boolean)
  if (tokens.length === 0) return false
  let pos = 0

  const parseOr = (): boolean => {
    let ok = parseAnd()
    while (tokens[pos] === 'OR') {
      pos++
      ok = parseAnd() || ok
    }
    return ok
  }
  const parseAnd = (): boolean => {
    let ok = parseTerm()
    while (tokens[pos] === 'AND') {
      pos++
      ok = parseTerm() && ok
    }
    return ok
  }
  const parseTerm = (): boolean => {
    const token = tokens[pos++]
    if (token === '(') {
      const ok = parseOr()
      if (tokens[pos++] !== ')') throw new Error(`licenses: unbalanced expression "${expression}"`)
      return ok
    }
    if (token === undefined || token === ')' || token === 'AND' || token === 'OR') {
      throw new Error(`licenses: malformed expression "${expression}"`)
    }
    if (tokens[pos] === 'WITH') pos += 2
    return ACCEPTED.has(token.replace(/\+$/, ''))
  }

  try {
    const ok = parseOr()
    return ok && pos === tokens.length
  } catch {
    return false
  }
}

// Go modules carry no license field, so the SPDX id is read from the license text. Order matters:
// a text matching an earlier signature is not tested against later, looser ones.
const SIGNATURES: [string, RegExp[]][] = [
  ['Apache-2.0', [/Apache License\s+Version 2\.0/i]],
  ['MPL-2.0', [/Mozilla Public License,?\s+(Version|v\.)\s*2\.0/i]],
  [
    'ISC',
    [/Permission to use, copy, modify, and(\/or)? distribute this software for any\s+purpose/i]
  ],
  ['MIT', [/Permission is hereby granted, free of charge/i]],
  [
    'BSD-3-Clause',
    [
      /Redistribution and use in source and binary forms/i,
      /(Neither the name|names of (its|the) contributors) may\s+(not\s+)?be used/i
    ]
  ],
  ['BSD-2-Clause', [/Redistribution and use in source and binary forms/i]]
]

export function detectLicense(text: string): string | null {
  const flat = text.replace(/\s+/g, ' ')
  for (const [id, patterns] of SIGNATURES) {
    if (patterns.every((p) => p.test(flat))) return id
  }
  return null
}
