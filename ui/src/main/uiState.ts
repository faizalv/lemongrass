// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { BrowserWindow, ipcMain, screen } from 'electron'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'fs'
import { homedir } from 'os'
import { join } from 'path'
import { windowBackground } from './appearance'
import { zoomPresets } from './zoomPresets'

export interface WindowState {
  x: number
  y: number
  width: number
  height: number
  maximized: boolean
  fullScreen: boolean
}

export type ThemeSetting = 'system' | 'dark' | 'light'

export interface UiState {
  lastProjectId?: string
  zoomFactor?: number
  startup?: 'lastProject' | 'projectSelector'
  // Where the header draws minimize, maximize and close on Linux and Windows. macOS ignores it and shows its own traffic lights.
  windowControls?: 'right' | 'left'
  // The app theme; system follows the OS. Dark when unset.
  theme?: ThemeSetting
  // Terminals stay dark unless set to follow the app, since an agent picks its own colors for a dark terminal.
  terminalTheme?: 'dark' | 'app'
  terminalFontSize?: number
  // The sidebar's Biblio or Files view, and the file explorer's view options.
  sidebarView?: 'biblio' | 'files'
  // The view the sidebar column shows, picked from the upper group of the side toolbar.
  sidebarPane?: 'files' | 'workgroup' | 'ssh'
  // The panel folded open under the column's view. Older files may hold 'workgroup', read as none.
  sidebarPanel?: 'git' | 'none'
  explorer?: {
    columns: { size: boolean; modified: boolean; git: boolean }
    density: 'compact' | 'comfortable'
    hideIgnored: boolean
  }
  window?: WindowState
  // The app version the "Optimizing lemongrass" layer last completed for. Main alone writes it, through deps:setupDone.
  setupVersion?: string
  // The last status of each required dependency; one that was ok and no longer is reopens the layer.
  setupSeen?: Record<string, string>
}

const STORE_DIR = join(homedir(), '.lemongrass')
const STORE_FILE = join(STORE_DIR, 'ui-state.json')

export function loadUiState(): UiState {
  if (!existsSync(STORE_FILE)) return {}
  try {
    return JSON.parse(readFileSync(STORE_FILE, 'utf-8'))
  } catch {
    return {}
  }
}

// Merges into the file, so each field is saved independently.
export function saveUiState(patch: UiState): void {
  mkdirSync(STORE_DIR, { recursive: true })
  writeFileSync(STORE_FILE, JSON.stringify({ ...loadUiState(), ...patch }, null, 2))
}

export function registerUiStateHandlers(getWindow: () => BrowserWindow | undefined): void {
  ipcMain.handle('uiState:load', () => loadUiState())
  ipcMain.on('uiState:save', (_event, patch: UiState) => {
    saveUiState(patch)
    if (patch.theme) getWindow()?.setBackgroundColor(windowBackground(patch.theme))
  })

  // Measured on the display the window is on now, so moving it to another screen changes the list.
  ipcMain.handle('uiState:zoomPresets', () => {
    const win = getWindow()
    const display = win ? screen.getDisplayMatching(win.getBounds()) : screen.getPrimaryDisplay()
    return zoomPresets(display.workAreaSize.width)
  })
}
