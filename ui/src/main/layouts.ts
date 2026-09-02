import { ipcMain } from 'electron'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'fs'
import { homedir } from 'os'
import { join } from 'path'

// Pane-layout persistence: the split/tab arrangement per project, not the
// live shell processes themselves. A restored leaf spawns a fresh shell
// into the same visual slot -- this never carries over scrollback or the
// running process, both of which end with the app regardless.

const STORE_DIR = join(homedir(), '.lemongrass')
const STORE_FILE = join(STORE_DIR, 'layouts.json')

function loadAll(): Record<string, unknown> {
  if (!existsSync(STORE_FILE)) return {}
  try {
    return JSON.parse(readFileSync(STORE_FILE, 'utf-8'))
  } catch {
    return {}
  }
}

function saveAll(layouts: Record<string, unknown>): void {
  mkdirSync(STORE_DIR, { recursive: true })
  writeFileSync(STORE_FILE, JSON.stringify(layouts, null, 2))
}

export function registerLayoutHandlers(): void {
  ipcMain.handle('layouts:load', (_event, projectId: string) => {
    return loadAll()[projectId] ?? null
  })

  ipcMain.on(
    'layouts:save',
    (_event, { projectId, layout }: { projectId: string; layout: unknown }) => {
      const all = loadAll()
      if (layout === null) delete all[projectId]
      else all[projectId] = layout
      saveAll(all)
    }
  )
}
