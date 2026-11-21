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

const TASK_POPUPS: Record<string, { title: string; body: (project: string) => string }> = {
  close: {
    title: 'Task ready to close',
    body: (project) =>
      `An agent finished its task in ${project}. Close it while its cache is still warm.`
  },
  idle: {
    title: 'Unfinished task',
    body: (project) =>
      `An agent in ${project} stopped working 15 minutes ago, but its task is not finished.`
  },
  approval: {
    title: 'Task waiting for approval',
    body: (project) => `A task in ${project} is finished and waiting for your approval.`
  }
}

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
  ipcMain.handle(
    'tabSessions:taskPopup',
    (event, projectPath: string, tabId: string, kind: string): void => {
      const window = BrowserWindow.fromWebContents(event.sender)
      const text = TASK_POPUPS[kind]
      if (!window || !text || typeof projectPath !== 'string' || !TAB_ID.test(tabId)) return
      notify(window, {
        title: text.title,
        body: text.body(basename(projectPath)),
        channel: 'notice:open-tab',
        payload: { tabId }
      })
    }
  )

  ipcMain.handle(
    'tabSessions:closeTask',
    async (_event, tabId: string, slug: string): Promise<PasteOutcome | 'invalid'> => {
      if (!SLUG.test(slug)) return 'invalid'
      return pastePrompt(tabId, `Close the task ${slug}.`)
    }
  )
}
