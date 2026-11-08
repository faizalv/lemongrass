// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { app } from 'electron'
import { spawn } from 'child_process'
import { appendFileSync, mkdirSync } from 'fs'
import { dirname } from 'path'

// Quits the app and starts a Windows installer as it exits. The 'quit' event comes after every window closed and
// window-all-closed stopped the shells and daemons. The program is started directly, as electron-updater does: an
// earlier hidden PowerShell hand-off never ran the setup on a real host. Electron-builder's installer retries
// whatever the exiting app still holds in the install folder. The log is the only trace once the app is gone, and a
// failed spawn is recorded synchronously, because the process exits before an 'error' event would fire.
export function quitAndRun(program: string, args: string[], log: string): void {
  app.once('quit', () => {
    let line: string
    try {
      const child = spawn(program, args, { detached: true, stdio: 'ignore' })
      line =
        child.pid === undefined
          ? `could not start ${program}`
          : `started ${program} ${args.join(' ')} as pid ${child.pid}`
      child.unref()
    } catch (err) {
      line = `could not start ${program}: ${err instanceof Error ? err.message : String(err)}`
    }
    try {
      mkdirSync(dirname(log), { recursive: true })
      appendFileSync(log, `${new Date().toISOString()} ${line}\n`)
    } catch {
      // Nothing is left to report to once the app is exiting.
    }
  })
  app.quit()
}
