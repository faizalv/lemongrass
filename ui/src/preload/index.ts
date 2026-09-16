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

export interface BiblioNode {
  name: string
  path: string
  type: 'dir' | 'file'
  children?: BiblioNode[]
}

export interface BiblioTree {
  toc: BiblioNode | null
  children: BiblioNode[]
}

export interface VaultScope {
  Tables: string[]
  Operations: string[]
}

export interface VaultChannel {
  ID: string
  Name: string
  DBName: string
  Scope: VaultScope
  CreatedAt: string
  ExpiresAt: string
}

export interface VaultChannelWithShortId {
  channel: VaultChannel
  shortId: string
}

export interface PaneTab {
  id: string
  label: string
  command: string
  cwd?: string
}

export type PaneLayoutNode =
  | { type: 'leaf'; id: string; tabs: PaneTab[]; activeTabId: string | null }
  | {
      type: 'split'
      id: string
      direction: 'row' | 'column'
      children: PaneLayoutNode[]
      sizes: number[]
    }

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

const layouts = {
  load: (projectId: string): Promise<PaneLayoutNode | null> =>
    ipcRenderer.invoke('layouts:load', projectId),
  save: (projectId: string, layout: PaneLayoutNode | null): void =>
    ipcRenderer.send('layouts:save', { projectId, layout })
}

const biblio = {
  tree: (projectPath: string): Promise<BiblioTree | null> =>
    ipcRenderer.invoke('biblio:tree', projectPath),
  read: (projectPath: string, relativePath: string): Promise<string | null> =>
    ipcRenderer.invoke('biblio:read', projectPath, relativePath),
  write: (projectPath: string, relativePath: string, content: string): Promise<boolean> =>
    ipcRenderer.invoke('biblio:write', projectPath, relativePath, content),
  createScratchpad: (projectPath: string, title: string, content: string): Promise<string | null> =>
    ipcRenderer.invoke('biblio:createScratchpad', projectPath, title, content),
  createScratchpadFile: (
    projectPath: string,
    folderPath: string,
    title: string,
    content: string
  ): Promise<string | null> =>
    ipcRenderer.invoke('biblio:createScratchpadFile', projectPath, folderPath, title, content),
  archiveScratchpad: (projectPath: string, relativePath: string): Promise<boolean> =>
    ipcRenderer.invoke('biblio:archiveScratchpad', projectPath, relativePath),
  onChanged: (callback: (projectPath: string) => void): (() => void) => {
    const listener = (_event: Electron.IpcRendererEvent, projectPath: string): void =>
      callback(projectPath)
    ipcRenderer.on('biblio:changed', listener)
    return () => ipcRenderer.removeListener('biblio:changed', listener)
  }
}

const vault = {
  list: (): Promise<VaultChannel[]> => ipcRenderer.invoke('vault:list'),
  create: (
    passphrase: string,
    name: string,
    dbName: string,
    scope: VaultScope,
    ttlSeconds: number
  ): Promise<VaultChannelWithShortId> =>
    ipcRenderer.invoke('vault:create', passphrase, name, dbName, scope, ttlSeconds),
  activate: (
    passphrase: string,
    id: string,
    ttlSeconds: number
  ): Promise<VaultChannelWithShortId> =>
    ipcRenderer.invoke('vault:activate', passphrase, id, ttlSeconds),
  revoke: (id: string): Promise<void> => ipcRenderer.invoke('vault:revoke', id),
  listConnections: (): Promise<string[]> => ipcRenderer.invoke('vault:listConnections'),
  putCredential: (passphrase: string, name: string, connectionString: string): Promise<void> =>
    ipcRenderer.invoke('vault:putCredential', passphrase, name, connectionString),
  deleteConnection: (name: string): Promise<void> =>
    ipcRenderer.invoke('vault:deleteConnection', name),
  testConnection: (connectionString: string): Promise<void> =>
    ipcRenderer.invoke('vault:testConnection', connectionString),
  testSavedConnection: (passphrase: string, name: string): Promise<void> =>
    ipcRenderer.invoke('vault:testSavedConnection', passphrase, name),
  listTables: (passphrase: string, name: string): Promise<string[]> =>
    ipcRenderer.invoke('vault:listTables', passphrase, name),
  hasPassphrase: (): Promise<boolean> => ipcRenderer.invoke('vault:hasPassphrase'),
  setPassphrase: (passphrase: string): Promise<void> =>
    ipcRenderer.invoke('vault:setPassphrase', passphrase),
  verifyPassphrase: (passphrase: string): Promise<void> =>
    ipcRenderer.invoke('vault:verifyPassphrase', passphrase),
  resetVault: (): Promise<void> => ipcRenderer.invoke('vault:resetVault')
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

const api = { pty, projects, layouts, biblio, vault, windowControls }

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
