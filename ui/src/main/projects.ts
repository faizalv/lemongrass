import { BrowserWindow, dialog, ipcMain } from 'electron'
import { existsSync, mkdirSync, readFileSync, realpathSync, writeFileSync } from 'fs'
import { homedir } from 'os'
import { basename, join } from 'path'
import { randomUUID } from 'crypto'

export interface Project {
  id: string
  name: string
  path: string
}

// Centralized under ~/.lemongrass/, not Electron's userData folder, so the CLI and UI share one registry.
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

// Mirrors project.resolvePath on the lgrass CLI side, so a symlinked path registered from either converges on the same entry.
function resolvePath(path: string): string {
  try {
    return realpathSync(path)
  } catch {
    return path
  }
}

export function registerProjectHandlers(getWindow: () => BrowserWindow | undefined): void {
  ipcMain.handle('projects:list', () => loadProjects())

  ipcMain.handle('projects:add', async () => {
    const win = getWindow()
    const result = win
      ? await dialog.showOpenDialog(win, { properties: ['openDirectory'] })
      : await dialog.showOpenDialog({ properties: ['openDirectory'] })
    if (result.canceled || result.filePaths.length === 0) return null

    const dirPath = resolvePath(result.filePaths[0])
    const projects = loadProjects()
    const existing = projects.find((p) => resolvePath(p.path) === dirPath)
    if (existing) return existing

    const project: Project = { id: randomUUID(), name: basename(dirPath), path: dirPath }
    projects.push(project)
    saveProjects(projects)
    return project
  })
}
