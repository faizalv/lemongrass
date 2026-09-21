import { ipcMain } from 'electron'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'fs'
import { homedir } from 'os'
import { join } from 'path'

const STORE_DIR = join(homedir(), '.lemongrass')
const STORE_FILE = join(STORE_DIR, 'doc-layouts.json')

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

export function registerDocLayoutHandlers(): void {
  ipcMain.handle('docLayouts:load', (_event, projectId: string) => {
    return loadAll()[projectId] ?? null
  })

  ipcMain.on(
    'docLayouts:save',
    (_event, { projectId, layout }: { projectId: string; layout: unknown }) => {
      const all = loadAll()
      if (layout === null) delete all[projectId]
      else all[projectId] = layout
      saveAll(all)
    }
  )
}
