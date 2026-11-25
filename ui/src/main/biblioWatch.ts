// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { lstatSync, readdirSync, watch, type FSWatcher } from 'fs'
import { join, sep } from 'path'

export interface TreeWatch {
  close(): void
}

// Calls onChange for any change below root, and onError once root itself can no longer be watched.
export function watchTree(root: string, onChange: () => void, onError: () => void): TreeWatch {
  if (process.platform === 'linux') return watchDirs(root, onChange, onError)
  const watcher = watch(root, { recursive: true }, onChange)
  watcher.on('error', onError)
  return { close: () => watcher.close() }
}

function isRealDir(path: string): boolean {
  try {
    return lstatSync(path).isDirectory()
  } catch {
    return false
  }
}

// Node's recursive watch on Linux watches each file by its inode, so a file replaced by
// write-then-rename drops out and its later in-place edits are never reported. A directory's own
// watch reports every change to the entries inside it whatever their inode, so this keeps one
// non-recursive watch per directory instead, following directories as they appear and go.
// Symlinked directories are not followed, so a link loop cannot recurse.
function watchDirs(root: string, onChange: () => void, onError: () => void): TreeWatch {
  const dirs = new Map<string, FSWatcher>()

  const drop = (dir: string): void => {
    for (const [path, watcher] of dirs) {
      if (path === dir || path.startsWith(dir + sep)) {
        watcher.close()
        dirs.delete(path)
      }
    }
  }

  const add = (dir: string): void => {
    if (dirs.has(dir)) return
    let watcher: FSWatcher
    try {
      watcher = watch(dir, (event, filename) => {
        onChange()
        if (event !== 'rename' || !filename) return
        const child = join(dir, filename.toString())
        if (isRealDir(child)) add(child)
        else drop(child)
      })
    } catch {
      if (dir === root) onError()
      return
    }
    watcher.on('error', () => {
      if (dir === root) onError()
      else drop(dir)
    })
    dirs.set(dir, watcher)
    // A directory created with content already inside it reports only itself, so its subdirectories
    // are found by listing it.
    let entries
    try {
      entries = readdirSync(dir, { withFileTypes: true })
    } catch {
      return
    }
    for (const entry of entries) if (entry.isDirectory()) add(join(dir, entry.name))
  }

  add(root)
  return {
    close: () => {
      for (const watcher of dirs.values()) watcher.close()
      dirs.clear()
    }
  }
}
