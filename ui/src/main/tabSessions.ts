// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { BrowserWindow, ipcMain } from 'electron'
import { execFile } from 'child_process'
import type { TabBusy, TabHookState, TabStateChange } from '../preload/types'

const LGRASS_TIMEOUT_MS = 5_000

function runLgrassd(lgrassdPath: string, projectPath: string, args: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(
      lgrassdPath,
      ['tabs', ...args],
      { cwd: projectPath, encoding: 'utf8', timeout: LGRASS_TIMEOUT_MS, windowsHide: true },
      (failure, stdout) => (failure ? reject(failure) : resolve(stdout))
    )
  })
}

let registeredLgrassdPath: string | null = null

export async function registerTab(
  projectPath: string,
  tabId: string,
  vendor: string
): Promise<void> {
  if (!registeredLgrassdPath) return
  try {
    await runLgrassd(registeredLgrassdPath, projectPath, ['register', tabId, vendor])
  } catch (err) {
    console.error('lgrassd tabs register failed:', err)
  }
}

const BUSY_REASONS: Record<string, string> = {
  working: 'Mid-turn',
  prompting: 'Permission prompt open'
}
let statesFailureLogged = false

export function hookState(state: unknown): TabHookState {
  return state === 'working' || state === 'prompting' ? state : 'idle'
}

// Tabs with no recorded state are left out and count as not busy. An lgrassd built before
// `tabs states` existed fails here, which also reads as nobody busy.
export async function tabStates(projectPath: string): Promise<Record<string, TabBusy>> {
  if (!registeredLgrassdPath) return {}
  let states: Record<string, string>
  try {
    states = JSON.parse(await runLgrassd(registeredLgrassdPath, projectPath, ['states']))
  } catch (err) {
    // The picker polls this, so a lasting failure is logged once.
    if (!statesFailureLogged) console.error('lgrassd tabs states failed:', err)
    statesFailureLogged = true
    return {}
  }
  const out: Record<string, TabBusy> = {}
  for (const [tabId, state] of Object.entries(states ?? {})) {
    const reason = BUSY_REASONS[state] ?? null
    out[tabId] = { busy: reason !== null, reason, state: hookState(state) }
  }
  return out
}

// Tabs a hook last reported busy, kept from the tab_state pushes passing through main, so a
// keystroke is checked against it without asking lgrassd.
const busyTabs = new Set<string>()
let stateWindow: () => BrowserWindow | undefined = () => undefined

export function setTabStateWindow(getWindow: () => BrowserWindow | undefined): void {
  stateWindow = getWindow
}

export function publishTabState(change: TabStateChange): void {
  if (change.state === 'idle') busyTabs.delete(change.tabId)
  else busyTabs.add(change.tabId)
  const window = stateWindow()
  if (window && !window.isDestroyed()) window.webContents.send('tabSessions:state-changed', change)
}

// The keys that interrupt an agent's turn, typed alone. An arrow key or a paste also starts with
// ESC but arrives longer, so it never matches.
export function isInterruptKey(data: string): boolean {
  return data === '\x03' || data === '\x1b'
}

// Claude Code runs no hook when the human interrupts a turn, so its tab would stay busy until its
// idle notification a minute later. The human's own interrupt key into a busy tab ends the turn in
// the store, which also makes the next turn's first hook a change that is pushed again.
export async function interruptTab(projectPath: string, tabId: string): Promise<void> {
  if (!registeredLgrassdPath || !busyTabs.has(tabId)) return
  try {
    const out = await runLgrassd(registeredLgrassdPath, projectPath, ['interrupted', tabId])
    if ((JSON.parse(out) as { changed?: boolean }).changed)
      publishTabState({ tabId, state: 'idle' })
  } catch (err) {
    console.error('lgrassd tabs interrupted failed:', err)
  }
}

export function registerTabSessionHandlers(lgrassdPath: string | null): void {
  registeredLgrassdPath = lgrassdPath
  ipcMain.handle(
    'tabSessions:list',
    async (_event, projectPath: string): Promise<Record<string, string>> => {
      if (!lgrassdPath) return {}
      try {
        return JSON.parse(await runLgrassd(lgrassdPath, projectPath, ['list']))
      } catch (err) {
        console.error('lgrassd tabs list failed:', err)
        return {}
      }
    }
  )

  ipcMain.handle('tabSessions:states', (_event, projectPath: string) => tabStates(projectPath))

  ipcMain.handle(
    'tabSessions:owners',
    async (_event, projectPath: string): Promise<Record<string, string>> => {
      if (!lgrassdPath) return {}
      try {
        return JSON.parse(await runLgrassd(lgrassdPath, projectPath, ['owners']))
      } catch (err) {
        console.error('lgrassd tabs owners failed:', err)
        return {}
      }
    }
  )

  ipcMain.handle('tabSessions:clear', async (_event, projectPath: string) => {
    if (!lgrassdPath) return
    try {
      await runLgrassd(lgrassdPath, projectPath, ['clear'])
    } catch (err) {
      console.error('lgrassd tabs clear failed:', err)
    }
  })

  ipcMain.handle(
    'tabSessions:title',
    async (
      _event,
      { projectPath, tabId, title }: { projectPath: string; tabId: string; title: string }
    ) => {
      if (!lgrassdPath) return
      try {
        await runLgrassd(lgrassdPath, projectPath, ['title', tabId, title])
      } catch (err) {
        console.error('lgrassd tabs title failed:', err)
      }
    }
  )

  ipcMain.handle(
    'tabSessions:forget',
    async (_event, { projectPath, tabId }: { projectPath: string; tabId: string }) => {
      if (!lgrassdPath) return
      try {
        await runLgrassd(lgrassdPath, projectPath, ['forget', tabId])
      } catch (err) {
        console.error('lgrassd tabs forget failed:', err)
      }
    }
  )
}
