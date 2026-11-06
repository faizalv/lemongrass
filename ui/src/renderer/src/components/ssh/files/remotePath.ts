// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Remote paths are POSIX and absolute whatever the local OS is, so they are joined here by hand
// instead of with node's path module, which the renderer does not have.

export interface Crumb {
  name: string
  path: string
}

export function joinRemote(dir: string, name: string): string {
  const base = dir.replace(/\/+$/, '')
  return `${base}/${name}`
}

export function parentOf(path: string): string {
  const trimmed = path.replace(/\/+$/, '')
  const cut = trimmed.lastIndexOf('/')
  return cut <= 0 ? '/' : trimmed.slice(0, cut)
}

export function crumbsOf(path: string): Crumb[] {
  const crumbs: Crumb[] = [{ name: '/', path: '/' }]
  let at = ''
  for (const part of path.split('/')) {
    if (!part) continue
    at = `${at}/${part}`
    crumbs.push({ name: part, path: at })
  }
  return crumbs
}

// The reason a typed name cannot be a file or folder name, or null when it can.
export function nameProblem(name: string): string | null {
  if (!name) return 'Type a name.'
  if (name === '.' || name === '..') return 'That name is reserved.'
  if (name.includes('/')) return 'A name cannot contain a slash.'
  return null
}
