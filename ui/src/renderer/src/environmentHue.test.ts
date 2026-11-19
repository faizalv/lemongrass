// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { ENVIRONMENT_HUES, environmentHue, environmentHueStyle } from './environmentHue.ts'

test('environments of one rester differ until the palette runs out', () => {
  const hues = ENVIRONMENT_HUES.map((_, i) => environmentHue(i))
  assert.equal(new Set(hues).size, ENVIRONMENT_HUES.length)
})

test('the hue wraps around past the palette', () => {
  assert.equal(environmentHue(ENVIRONMENT_HUES.length), environmentHue(0))
})

test('the style names the theme color token of the hue', () => {
  assert.deepEqual(environmentHueStyle(0), { '--env-hue': 'var(--color-sky)' })
})
