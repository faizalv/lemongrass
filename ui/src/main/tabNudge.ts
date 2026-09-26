import { execFile } from 'child_process'
import { findShellByTab } from './pty'

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/
const LGRASS_TIMEOUT_MS = 5_000

export interface NudgeReply {
  ok: boolean
  typed?: boolean
  reason?: string
  error?: string
}

function composeNudge(lgrassPath: string, cwd: string, tabId: string): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(
      lgrassPath,
      ['session', 'nudge', tabId],
      { cwd, encoding: 'utf8', timeout: LGRASS_TIMEOUT_MS },
      (failure, stdout) => (failure ? reject(failure) : resolve(stdout))
    )
  })
}

function printable(text: string): string {
  return [...text]
    .map((ch) => (ch.charCodeAt(0) < 32 || ch.charCodeAt(0) === 127 ? ' ' : ch))
    .join('')
    .trim()
}

// The request carries only a tab id, so the typed line always comes from lgrass's own ledger and no caller can type text of its own choosing.
export async function handleNudge(lgrassPath: string | null, tabId: string): Promise<NudgeReply> {
  if (!lgrassPath) return { ok: false, error: 'lgrass is not installed' }
  if (!UUID.test(tabId)) return { ok: false, error: 'the tab id is not valid' }
  const shell = findShellByTab(tabId)
  if (!shell) return { ok: false, error: 'no open tab has that id' }
  if (shell.humanIsTyping()) return { ok: true, typed: false, reason: 'typing' }

  const text = printable(await composeNudge(lgrassPath, shell.cwd, tabId))
  if (!text) return { ok: true, typed: false, reason: 'nothing pending' }
  shell.type(text + '\r')
  return { ok: true, typed: true }
}
