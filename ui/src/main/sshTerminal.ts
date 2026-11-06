// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { mkdirSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'
import { ALIAS_PATTERN } from './sshHosts'

// The arguments of a server tab's `ssh`. The terminal owns a ControlMaster connection, so a
// password, passphrase or host key prompt is answered once there, and the file and container
// panels open their channels over the same socket later. The socket sits under the lemongrass
// home, which the danger gate keeps models out of.
export function terminalArgs(alias: string): string[] {
  if (!ALIAS_PATTERN.test(alias)) throw new Error(`ssh: "${alias}" is not a valid server name.`)
  const dir = join(homedir(), '.lemongrass', 'ssh')
  mkdirSync(dir, { recursive: true, mode: 0o700 })
  return [
    '-o',
    'ControlMaster=auto',
    '-o',
    `ControlPath=${join(dir, 'cm-%C')}`,
    '-o',
    'ControlPersist=60',
    '--',
    alias
  ]
}
