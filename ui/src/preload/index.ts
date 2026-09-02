import { contextBridge, ipcRenderer } from 'electron'
import { electronAPI } from '@electron-toolkit/preload'

export interface PtySpawnOptions {
  command: string
  args?: string[]
  cwd?: string
  cols?: number
  rows?: number
}

export interface PtyDataPayload {
  id: string
  data: string
}

export interface PtyExitPayload {
  id: string
  exitCode: number
  signal?: number
}

export interface Project {
  id: string
  name: string
  path: string
}

export interface TreeNode {
  name: string
  path: string
  type: 'file' | 'dir'
  children?: TreeNode[]
}

export interface PaneTab {
  id: string
  label: string
  command: string
  cwd?: string
}

export type PaneLayoutNode =
  | { type: 'leaf'; id: string; tabs: PaneTab[]; activeTabId: string | null }
  | { type: 'split'; id: string; direction: 'row' | 'column'; children: PaneLayoutNode[]; sizes: number[] }

// Custom APIs for renderer -- raw PTY bytes only, never a control-signal
// channel. PTY is display-only.
const pty = {
  spawn: (opts: PtySpawnOptions): Promise<{ id: string }> => ipcRenderer.invoke('pty:spawn', opts),
  write: (id: string, data: string): void => ipcRenderer.send('pty:write', { id, data }),
  resize: (id: string, cols: number, rows: number): void =>
    ipcRenderer.send('pty:resize', { id, cols, rows }),
  kill: (id: string): void => ipcRenderer.send('pty:kill', { id }),
  onData: (callback: (payload: PtyDataPayload) => void): (() => void) => {
    const listener = (_event: Electron.IpcRendererEvent, payload: PtyDataPayload): void =>
      callback(payload)
    ipcRenderer.on('pty:data', listener)
    return () => ipcRenderer.removeListener('pty:data', listener)
  },
  onExit: (callback: (payload: PtyExitPayload) => void): (() => void) => {
    const listener = (_event: Electron.IpcRendererEvent, payload: PtyExitPayload): void =>
      callback(payload)
    ipcRenderer.on('pty:exit', listener)
    return () => ipcRenderer.removeListener('pty:exit', listener)
  }
}

const projects = {
  list: (): Promise<Project[]> => ipcRenderer.invoke('projects:list'),
  add: (): Promise<Project | null> => ipcRenderer.invoke('projects:add')
}

const knowledge = {
  tree: (projectPath: string): Promise<TreeNode[]> =>
    ipcRenderer.invoke('knowledge:tree', projectPath),
  read: (projectPath: string, filePath: string): Promise<string> =>
    ipcRenderer.invoke('knowledge:read', { projectPath, filePath })
}

const layouts = {
  load: (projectId: string): Promise<PaneLayoutNode | null> =>
    ipcRenderer.invoke('layouts:load', projectId),
  save: (projectId: string, layout: PaneLayoutNode | null): void =>
    ipcRenderer.send('layouts:save', { projectId, layout })
}

const windowControls = {
  minimize: (): void => ipcRenderer.send('window:minimize'),
  toggleMaximize: (): void => ipcRenderer.send('window:toggle-maximize'),
  close: (): void => ipcRenderer.send('window:close'),
  isMaximized: (): Promise<boolean> => ipcRenderer.invoke('window:is-maximized'),
  onMaximizedChange: (callback: (maximized: boolean) => void): (() => void) => {
    const listener = (_event: Electron.IpcRendererEvent, maximized: boolean): void =>
      callback(maximized)
    ipcRenderer.on('window:maximized', listener)
    return () => ipcRenderer.removeListener('window:maximized', listener)
  }
}

const api = { pty, projects, knowledge, layouts, windowControls }

if (process.contextIsolated) {
  try {
    contextBridge.exposeInMainWorld('electron', electronAPI)
    contextBridge.exposeInMainWorld('api', api)
  } catch (error) {
    console.error(error)
  }
} else {
  // @ts-ignore (define in dts)
  window.electron = electronAPI
  // @ts-ignore (define in dts)
  window.api = api
}
