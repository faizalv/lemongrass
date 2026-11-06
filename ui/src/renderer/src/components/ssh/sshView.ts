// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { SSHHostEntry, SSHHostList, SSHKey } from '../../../../preload/types'

export type SSHSection = 'servers' | 'keys'

export function hostTarget(host: SSHHostEntry): string {
  const r = host.resolved
  if (!r) return ''
  const port = r.port && r.port !== '22' ? `:${r.port}` : ''
  return `${r.user ? r.user + '@' : ''}${r.hostname}${port}`
}

export function tildePath(path: string, home: string): string {
  return home && path.startsWith(home + '/') ? '~' + path.slice(home.length) : path
}

// The folder the human's ssh config lives in, where the pane lists keys from.
export function sshDirOf(hosts: SSHHostList): string {
  return hosts.configFile.replace(/\/[^/]*$/, '')
}

// Servers whose config names the key as a login key.
export function serversUsing(key: SSHKey, hosts: SSHHostEntry[]): SSHHostEntry[] {
  return hosts.filter((h) => h.keyPaths.includes(key.privatePath))
}

// A login key the server names inside the ssh folder that is not there any more. A key file
// outside the folder is not listed by the pane, so it cannot be judged missing.
export function missingKeyPath(host: SSHHostEntry, keys: SSHKey[], sshDir: string): string | null {
  for (const path of host.keyPaths) {
    if (!path.startsWith(sshDir + '/')) continue
    if (!keys.some((k) => k.privatePath === path && k.hasPrivate)) return path
  }
  return null
}
