// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { BrowserWindow, ipcMain, screen } from 'electron'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'fs'
import { homedir } from 'os'
import { join } from 'path'
import { zoomPresets } from './zoomPresets'

export interface UiState {
  lastProjectId?: string
  zoomFactor?: number
}

const STORE_DIR = join(homedir(), '.lemongrass')
const STORE_FILE = join(STORE_DIR, 'ui-state.json')

function loadState(): UiState {
  if (!existsSync(STORE_FILE)) return {}
  try {
    return JSON.parse(readFileSync(STORE_FILE, 'utf-8'))
  } catch {
    return {}
  }
}

export function registerUiStateHandlers(getWindow: () => BrowserWindow | undefined): void {
  ipcMain.handle('uiState:load', () => loadState())

  // Merges into the file, so the project and the zoom are saved independently.
  ipcMain.on('uiState:save', (_event, patch: UiState) => {
    mkdirSync(STORE_DIR, { recursive: true })
    writeFileSync(STORE_FILE, JSON.stringify({ ...loadState(), ...patch }, null, 2))
  })

  // Measured on the display the window is on now, so moving it to another screen changes the list.
  ipcMain.handle('uiState:zoomPresets', () => {
    const win = getWindow()
    const display = win ? screen.getDisplayMatching(win.getBounds()) : screen.getPrimaryDisplay()
    return zoomPresets(display.workAreaSize.width)
  })
}
