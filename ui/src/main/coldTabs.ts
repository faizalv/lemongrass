// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { ipcMain, type WebContents } from 'electron'
import { isColdAndLarge } from '../preload/cacheState'
import { pastePrompt, setNudgeHold, type PasteOutcome } from './tabNudge'
import { contextOfTab } from './usage'

const SLUG = /^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$/

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

  ipcMain.handle(
    'tabSessions:closeTask',
    async (_event, tabId: string, slug: string): Promise<PasteOutcome | 'invalid'> => {
      if (!SLUG.test(slug)) return 'invalid'
      return pastePrompt(tabId, `Close the task ${slug}.`)
    }
  )
}
