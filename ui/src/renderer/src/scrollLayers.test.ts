// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'

// A scroll container with no background of its own is painted into the window layer and repainted
// on every scroll step. will-change: scroll-position gives it a layer of its own, so a scroll only
// moves it. A container with a solid background of its own is composited without the hint. These
// are small text boxes that scroll only an overflowing line or two.
const TEXT_BOXES = new Set([
  '.conflict-error',
  '.push-error-text',
  '.detail-message',
  '.branch-error-text',
  '.stash-error',
  '.git-error',
  '.forecast-paths'
])

const ROOTS = [
  join(import.meta.dirname, '.'),
  join(import.meta.dirname, '../../../../sankasten/src')
]

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name)
    if (statSync(path).isDirectory()) return sourceFiles(path)
    return /\.(vue|css)$/.test(name) ? [path] : []
  })
}

function styleText(path: string, text: string): string {
  const css = path.endsWith('.vue')
    ? [...text.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map((m) => m[1]).join('\n')
    : text
  return css.replace(/\/\*[\s\S]*?\*\//g, '')
}

test('every scroll container without a background of its own gets a compositor layer', () => {
  const missing: string[] = []
  for (const root of ROOTS) {
    for (const path of sourceFiles(root)) {
      const css = styleText(path, readFileSync(path, 'utf8'))
      for (const rule of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
        const body = rule[2]
        if (!/overflow(-y)?\s*:\s*(auto|scroll)\s*;/.test(body)) continue
        if (
          /background(-color)?\s*:/.test(body) ||
          /will-change\s*:\s*scroll-position/.test(body)
        ) {
          continue
        }
        const selector = rule[1].trim().split(/\s+/).pop() ?? ''
        if (TEXT_BOXES.has(selector)) continue
        missing.push(`${path.replace(root, '')}: ${rule[1].trim().replace(/\s+/g, ' ')}`)
      }
    }
  }
  assert.deepEqual(missing, [])
})
