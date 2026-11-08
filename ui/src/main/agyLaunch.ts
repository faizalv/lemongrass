// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { randomUUID } from 'node:crypto'
import { existsSync } from 'node:fs'
import { join } from 'node:path'

// What an Antigravity tab is launched with on top of its own arguments.
export interface AgyLaunch {
  args: string[]
  env: Record<string, string>
}

// Set only on a launch that resumes a conversation. SessionStart does not fire on `--conversation`, so the hook renews
// the laws at the first prompt of such a process, and the value differs per launch so it renews exactly once each.
export const AGY_RESUMED_ENV = 'LGRASS_AGY_RESUMED'

const SHELL_SAFE = /^[A-Za-z0-9/._\-+@%:,=]+$/
const CMD_SPECIAL = /[ \t&|<>()^,;=]/g

// Written the way agentconf.CmdHookCommand writes it, so the statusline command reads like the agy hooks the keeper
// registers. On Windows agy runs it as `cmd /c` through Go's os/exec, which turns a `"` into `\"` that cmd keeps, so
// the path is never quoted there: cmd's special characters get a caret instead.
export function quoteCommandPath(path: string, platform: NodeJS.Platform): string {
  if (platform === 'win32') return path.replace(/\\/g, '/').replace(CMD_SPECIAL, '^$&')
  return SHELL_SAFE.test(path) ? path : `'${path.replace(/'/g, `'\\''`)}'`
}

// Settings passed per session are never saved by agy, so lemongrass writes nothing into the user's own settings.json.
// The trusted project skips the trust dialog, which would otherwise catch a thinker's first prompt. The statusline is
// left to the user's own while lgrassd is missing or the human switched the chained statusline off, the same file
// that switches it off for Claude.
export function agyLaunch(
  args: string[],
  cwd: string | undefined,
  lgrassdPath: string | null,
  home: string,
  platform: NodeJS.Platform = process.platform
): AgyLaunch {
  const settings: Record<string, unknown> = {}
  if (cwd) settings.trustedWorkspaces = [cwd]
  const chainOff = existsSync(join(home, '.lemongrass', 'statusline-off'))
  if (lgrassdPath && !chainOff) {
    settings.statusLine = {
      type: 'command',
      command: `${quoteCommandPath(lgrassdPath, platform)} statusline --vendor agy`
    }
  }
  const resumed = args.includes('--conversation')
  return {
    args: Object.keys(settings).length > 0 ? ['--settings', JSON.stringify(settings)] : [],
    env: resumed ? { [AGY_RESUMED_ENV]: randomUUID() } : {}
  }
}
