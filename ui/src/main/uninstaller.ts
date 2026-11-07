// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { app, BrowserWindow, ipcMain } from 'electron'
import { execFile } from 'child_process'
import { existsSync } from 'fs'
import { homedir } from 'os'
import { dirname, join } from 'path'
import type { UninstallState } from '../preload/types'
import { resolveBundledBin } from './lgrassInstall'
import { runUntilExec } from './updateRun'
import { unsupportedReason as rpmUnsupportedReason } from './updaterRpm'
import { WIN_UNINSTALLER } from './updaterWinScript'
import { quitAndRun } from './winAfterQuit'

const RPM = '/usr/bin/rpm'

let currentWindow: () => BrowserWindow | undefined = () => undefined
let state: UninstallState = { phase: 'unavailable', message: null, output: null }

function set(patch: Partial<UninstallState>): void {
  state = { ...state, ...patch }
  currentWindow()?.webContents.send('uninstall:state-changed', state)
}

// The bundled copy is used rather than the one on PATH, which uninstall itself removes.
function bundledLgrassconf(): string | null {
  const path = resolveBundledBin('lgrassconf')
  return path && existsSync(path) ? path : null
}

function clean(binary: string): Promise<{ ok: boolean; output: string }> {
  return new Promise((resolve) => {
    execFile(binary, ['uninstall'], { encoding: 'utf8' }, (failure, stdout, stderr) => {
      resolve({ ok: !failure, output: `${stdout}${stderr}`.trim() })
    })
  })
}

function winUninstaller(): string {
  return join(dirname(app.getPath('exe')), WIN_UNINSTALLER)
}

async function unsupportedReason(): Promise<string | null> {
  if (process.platform === 'win32') {
    return existsSync(winUninstaller()) ? null : 'This copy was not installed from the setup.'
  }
  return rpmUnsupportedReason()
}

// The uninstaller starts as the app exits and kills whatever still runs from the install folder. Its own first step
// runs the bundled `lgrassconf uninstall` (ui/build/installer.nsh), so the account is cleaned with the app and its
// lgrassd daemons gone, which Windows needs to delete their binaries. The log goes in the lemongrass home, which
// uninstall keeps.
function uninstallWindows(): void {
  set({ phase: 'removing', message: null, output: null })
  quitAndRun(
    winUninstaller(),
    ['/S', '/currentuser'],
    join(homedir(), '.lemongrass', 'uninstall.log')
  )
}

// The per-user cleanup runs before the package goes, so a dismissed password prompt leaves only a cleaned account,
// which the next launch sets up again, and the app never keeps running on a deleted /opt.
async function uninstall(): Promise<void> {
  if (!['idle', 'failed'].includes(state.phase)) return
  if (process.platform === 'win32') {
    uninstallWindows()
    return
  }
  const binary = bundledLgrassconf()
  if (!binary) {
    set({ phase: 'failed', message: 'uninstall: the bundled lgrassconf is missing.', output: null })
    return
  }
  set({ phase: 'cleaning', message: null, output: null })
  const result = await clean(binary)
  if (!result.ok) {
    set({
      phase: 'failed',
      message: 'Cleaning up this account failed, so the package was not removed.',
      output: result.output
    })
    return
  }
  set({ phase: 'authorizing' })
  try {
    await runUntilExec('pkexec', [RPM, '-e', 'lemongrass'], () => set({ phase: 'removing' }))
  } catch (err) {
    const dismissed = (err as { code?: unknown }).code === 126
    set({
      phase: 'failed',
      message: dismissed
        ? 'The password prompt was dismissed, so the package stays. The next launch sets Lemongrass up again.'
        : `uninstall: ${err instanceof Error ? err.message : String(err)}`,
      output: result.output
    })
    return
  }
  app.quit()
}

export function registerUninstallHandlers(getWindow: () => BrowserWindow | undefined): void {
  currentWindow = getWindow
  ipcMain.handle('uninstall:state', (): UninstallState => state)
  ipcMain.handle('uninstall:run', (): void => {
    void uninstall()
  })
  if (!['linux', 'win32'].includes(process.platform) || !app.isPackaged || !bundledLgrassconf())
    return
  void unsupportedReason().then((reason) => {
    if (!reason) set({ phase: 'idle' })
  })
}
