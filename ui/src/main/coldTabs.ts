// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { BrowserWindow, ipcMain, type WebContents } from 'electron'
import { basename } from 'path'
import { notify } from './appNotify'
import { isColdAndLarge } from '../preload/cacheState'
import { pastePrompt, setNudgeHold, type PasteOutcome } from './tabNudge'
import { contextOfTab } from './usage'

const SLUG = /^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$/
const TAB_ID = /^[A-Za-z0-9-]{1,64}$/

// A tab whose prompt cache has gone cold over a large context gets no nudge typed into it, and the
// renderer is told so its warning can say a message waits. The close prompt is composed here from
// the task slug, so the renderer never sends text of its own.
export function registerColdTabHandlers(getSender: () => WebContents | undefined): void {
  setNudgeHold((tabId) => {
    if (!isColdAndLarge(contextOfTab(tabId), Date.now())) return false
    const sender = getSender()
    if (sender && !sender.isDestroyed()) sender.send('tabSessions:nudge-held', { tabId })
    return true
  })

  // The popup text is fixed apart from the project's folder name: a tab title or task slug is
  // model-written and never leaves the app. The center item, with the tab named, is the renderer's.
  ipcMain.handle('tabSessions:closeDuePopup', (event, projectPath: string, tabId: string): void => {
    const window = BrowserWindow.fromWebContents(event.sender)
    if (!window || typeof projectPath !== 'string' || !TAB_ID.test(tabId)) return
    notify(window, {
      title: 'Task ready to close',
      body: `An agent finished its task in ${basename(projectPath)}. Close it while its cache is still warm.`,
      channel: 'notice:open-tab',
      payload: { tabId }
    })
  })

  ipcMain.handle(
    'tabSessions:closeTask',
    async (_event, tabId: string, slug: string): Promise<PasteOutcome | 'invalid'> => {
      if (!SLUG.test(slug)) return 'invalid'
      return pastePrompt(tabId, `Close the task ${slug}.`)
    }
  )
}
