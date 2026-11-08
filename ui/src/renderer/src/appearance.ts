// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { computed, reactive } from 'vue'
import type { ThemeSetting } from '../../preload/types'

export type { ThemeSetting }
export type Scheme = 'dark' | 'light'
// dark keeps every terminal dark, app makes terminals follow the app theme.
export type TerminalTheme = 'dark' | 'app'

export const TERMINAL_FONT_SIZE = { min: 9, max: 24, fallback: 13 }

export const appearance = reactive({
  theme: window.api.uiState.initialTheme,
  scheme: 'dark' as Scheme,
  terminalTheme: 'dark' as TerminalTheme,
  terminalFontSize: TERMINAL_FONT_SIZE.fallback
})

// The scheme terminals draw in. An agent picks its own colors, and most assume a dark terminal,
// so terminals stay dark unless the human chose to have them follow the app.
export const terminalScheme = computed<Scheme>(() =>
  appearance.terminalTheme === 'app' ? appearance.scheme : 'dark'
)

const systemDark = window.matchMedia('(prefers-color-scheme: dark)')

function applyScheme(): void {
  const theme = appearance.theme
  appearance.scheme = theme === 'system' ? (systemDark.matches ? 'dark' : 'light') : theme
  document.documentElement.dataset.theme = appearance.scheme
}

// Runs before the app mounts, from the theme preload received, so the first paint is in the
// saved theme.
export function applyInitialTheme(): void {
  applyScheme()
  systemDark.addEventListener('change', () => {
    if (appearance.theme === 'system') applyScheme()
  })
}

function clampFontSize(size: number | undefined): number {
  if (size === undefined || !Number.isFinite(size)) return TERMINAL_FONT_SIZE.fallback
  return Math.min(TERMINAL_FONT_SIZE.max, Math.max(TERMINAL_FONT_SIZE.min, Math.round(size)))
}

let loaded: Promise<void> | undefined

export function loadAppearance(): Promise<void> {
  loaded ??= window.api.uiState.load().then((state) => {
    appearance.theme = state.theme ?? 'dark'
    appearance.terminalTheme = state.terminalTheme ?? 'dark'
    appearance.terminalFontSize = clampFontSize(state.terminalFontSize)
    applyScheme()
  })
  return loaded
}

export function setTheme(theme: ThemeSetting): void {
  appearance.theme = theme
  applyScheme()
  window.api.uiState.save({ theme })
}

export function setTerminalTheme(terminalTheme: TerminalTheme): void {
  appearance.terminalTheme = terminalTheme
  window.api.uiState.save({ terminalTheme })
}

export function setTerminalFontSize(size: number): void {
  const terminalFontSize = clampFontSize(size)
  appearance.terminalFontSize = terminalFontSize
  window.api.uiState.save({ terminalFontSize })
}
