// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { app } from 'electron'
import { existsSync } from 'fs'
import { dirname, join } from 'path'
import { winAssetName } from './updateVersion'
import { WIN_UNINSTALLER, winUnsupportedReason } from './updaterWinScript'
import type { UpdatePlatform } from './updatePlatform'
import { quitAndRun } from './winAfterQuit'

// Windows cannot replace the files of a running app, so installing only keeps the verified setup.exe, and restart
// starts it as the app exits. It runs without /S so its own progress window shows the update, and opens the new
// version when done. --updated, which electron-updater passes too, marks it as an update of a quitting app, so it does
// not ask about the running app and does not recreate a desktop shortcut the user deleted.

let staged: string | null = null

function installDir(): string {
  return dirname(app.getPath('exe'))
}

async function install(
  file: string,
  version: string,
  onInstalling: () => void
): Promise<string | null> {
  onInstalling()
  staged = file
  return `Version ${version} is ready. Restart to install it. Restarting closes every open tab.`
}

function restart(): void {
  if (!staged) return
  quitAndRun(staged, ['--updated', '--force-run'], join(dirname(staged), 'update.log'))
}

export const winPlatform: UpdatePlatform = {
  unsupportedReason: async () =>
    winUnsupportedReason(process.arch, existsSync(join(installDir(), WIN_UNINSTALLER))),
  assetName: winAssetName,
  installingMessage: 'Preparing the new version.',
  install,
  // The setup.exe runs only at restart, so it stays in its version's folder until the new version starts.
  keepAfterInstall: true,
  restart
}
