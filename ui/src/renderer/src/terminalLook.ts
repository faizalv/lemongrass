// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { ITerminalOptions } from '@xterm/xterm'
import { appearance, loadAppearance, terminalScheme, type Scheme } from './appearance'

// The font and colors every xterm in the app draws with, read from the design tokens. xterm keeps
// them as plain values, so a change of theme or size has to be set on each terminal again.

// A hidden element per scheme. A terminal that stays dark inside a light app needs the dark token
// values, and <html> only holds the app's scheme.
const probes = new Map<Scheme, HTMLElement>()

function schemeStyle(scheme: Scheme): CSSStyleDeclaration {
  let probe = probes.get(scheme)
  if (!probe) {
    probe = document.createElement('div')
    probe.dataset.theme = scheme
    probe.hidden = true
    document.body.appendChild(probe)
    probes.set(scheme, probe)
  }
  return getComputedStyle(probe)
}

function cssVar(name: string, scheme: Scheme = terminalScheme.value): string {
  return schemeStyle(scheme).getPropertyValue(name).trim()
}

const FONT_WAIT_MS = 3000
let fontReady: Promise<unknown> | undefined

// xterm measures its cell once, at open, from whichever face is available then, and never again
// when a font finishes loading. A terminal opened on the fallback face keeps a row count that no
// longer fits once the real one draws. The timeout keeps a font that fails to load from blocking
// every terminal. The saved font size is loaded first, so a terminal's first fit uses it.
export function terminalFontReady(): Promise<unknown> {
  fontReady ??= loadAppearance()
    .catch(() => undefined)
    .then(() =>
      Promise.race([
        document.fonts.load(`${appearance.terminalFontSize}px ${cssVar('--font-mono')}`),
        new Promise((resolve) => setTimeout(resolve, FONT_WAIT_MS))
      ])
    )
  return fontReady
}

// background overrides the default surface, for a terminal set inside a surface of its own color.
export function terminalLook(background?: string): ITerminalOptions {
  const scheme = terminalScheme.value
  background ||= cssVar('--color-surface-1', scheme)
  const color = (name: string): string => cssVar(name, scheme)
  return {
    fontFamily: cssVar('--font-mono'),
    fontSize: appearance.terminalFontSize,
    fontWeight: 400,
    fontWeightBold: 600,
    theme: {
      background,
      foreground: color('--color-fg-primary'),
      cursor: color('--color-amber'),
      cursorAccent: background,
      selectionBackground: color('--color-amber-muted'),
      black: color('--color-gray-700'),
      red: color('--color-coral-dim'),
      green: color('--color-jade-dim'),
      yellow: color('--color-amber-dim'),
      blue: color('--color-sapphire-dim'),
      magenta: color('--color-violet-dim'),
      cyan: color('--color-teal-dim'),
      white: color('--color-gray-100'),
      brightBlack: color('--color-gray-500'),
      brightRed: color('--color-coral'),
      brightGreen: color('--color-jade'),
      brightYellow: color('--color-amber'),
      brightBlue: color('--color-sapphire'),
      brightMagenta: color('--color-violet'),
      brightCyan: color('--color-teal'),
      brightWhite: color('--color-white')
    }
  }
}
