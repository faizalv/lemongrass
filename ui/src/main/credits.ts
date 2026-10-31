// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { ipcMain, shell } from 'electron'
import { existsSync, readFileSync } from 'fs'
import { dirname, join } from 'path'
import type { CreditsResult } from '../preload/types'

// scripts/licenses.ts writes licenses.json into out/ beside out/main, so this path holds in the
// packaged app.asar and in a make dev snapshot alike.
const LICENSES_FILE = join(__dirname, '../licenses.json')

// Electron ships Chromium's notices beside the executable, both packaged and under node_modules in
// dev. The list is far too large to copy into licenses.json, so it is opened where it is.
const CHROMIUM_LICENSES = 'LICENSES.chromium.html'

let cached: CreditsResult | null = null

function loadCredits(): CreditsResult {
  if (cached) return cached
  cached = existsSync(LICENSES_FILE)
    ? { entries: JSON.parse(readFileSync(LICENSES_FILE, 'utf8')), missing: false }
    : { entries: [], missing: true }
  return cached
}

// Resolves to an error message, or an empty string once the file is open.
async function openChromiumLicenses(): Promise<string> {
  const file = join(dirname(process.execPath), CHROMIUM_LICENSES)
  if (!existsSync(file)) return `credits: ${file} does not exist`
  return shell.openPath(file)
}

export function registerCreditsHandlers(): void {
  ipcMain.handle('credits:list', (): CreditsResult => loadCredits())
  ipcMain.handle('credits:openChromiumLicenses', () => openChromiumLicenses())
}
