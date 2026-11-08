// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { BrowserWindow, nativeTheme } from 'electron'
import { loadUiState, type ThemeSetting } from './uiState'

// The page color of each theme, --color-surface-0 in the Sankasten tokens. The window shows it
// before the page paints and while a resize outruns the page, so it must match the theme.
const PAGE_COLOR = { dark: '#0A0A0A', light: '#F2F0EA' } as const

// nativeTheme.themeSource stays at its default: setting it would also flip prefers-color-scheme in
// the web tester's pages, which should keep the OS scheme whatever the app's theme is.
function resolveTheme(setting: ThemeSetting): 'dark' | 'light' {
  if (setting === 'system') return nativeTheme.shouldUseDarkColors ? 'dark' : 'light'
  return setting
}

function savedTheme(): ThemeSetting {
  return loadUiState().theme ?? 'dark'
}

export function windowBackground(setting = savedTheme()): string {
  return PAGE_COLOR[resolveTheme(setting)]
}

// Passed to preload, so the renderer applies the theme synchronously before its first paint; an
// async ui-state load could lose that race and show the other theme first.
export function themeArgument(): string {
  return `--lg-theme=${savedTheme()}`
}

export function watchSystemTheme(getWindow: () => BrowserWindow | undefined): void {
  nativeTheme.on('updated', () => {
    if (savedTheme() === 'system') getWindow()?.setBackgroundColor(windowBackground('system'))
  })
}
