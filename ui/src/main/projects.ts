import { BrowserWindow, dialog, ipcMain } from 'electron'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'fs'
import { homedir } from 'os'
import { basename, join } from 'path'
import { randomUUID } from 'crypto'

export interface Project {
  id: string
  name: string
  path: string
}

// Lemongrass's own runtime state -- which projects it knows about --
// lives centrally under ~/.lemongrass/, not inside any project's own
// repo and not Electron's generic userData folder. Deliberate departure
// from the kencana context/ convention.
const STORE_DIR = join(homedir(), '.lemongrass')
const STORE_FILE = join(STORE_DIR, 'projects.json')

function loadProjects(): Project[] {
  if (!existsSync(STORE_FILE)) return []
  try {
    return JSON.parse(readFileSync(STORE_FILE, 'utf-8'))
  } catch {
    return []
  }
}

function saveProjects(projects: Project[]): void {
  mkdirSync(STORE_DIR, { recursive: true })
  writeFileSync(STORE_FILE, JSON.stringify(projects, null, 2))
}

export function registerProjectHandlers(getWindow: () => BrowserWindow | undefined): void {
  ipcMain.handle('projects:list', () => loadProjects())

  ipcMain.handle('projects:add', async () => {
    const win = getWindow()
    const result = win
      ? await dialog.showOpenDialog(win, { properties: ['openDirectory'] })
      : await dialog.showOpenDialog({ properties: ['openDirectory'] })
    if (result.canceled || result.filePaths.length === 0) return null

    const dirPath = result.filePaths[0]
    const projects = loadProjects()
    const existing = projects.find((p) => p.path === dirPath)
    if (existing) return existing

    const project: Project = { id: randomUUID(), name: basename(dirPath), path: dirPath }
    projects.push(project)
    saveProjects(projects)
    return project
  })
}
