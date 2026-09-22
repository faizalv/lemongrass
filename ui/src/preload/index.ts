import { contextBridge, ipcRenderer } from 'electron'
import { electronAPI } from '@electron-toolkit/preload'
import type {
  PtySpawnOptions,
  PtyDataPayload,
  PtyExitPayload,
  Project,
  BiblioTree,
  ScratchpadImageResult,
  ImageFileResult,
  VaultScope,
  VaultChannel,
  VaultChannelWithShortId,
  WorkspaceLayoutState
} from './types'

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

const workspaceLayouts = {
  load: (projectId: string): Promise<WorkspaceLayoutState | null> =>
    ipcRenderer.invoke('workspaceLayouts:load', projectId),
  save: (projectId: string, layout: WorkspaceLayoutState | null): void =>
    ipcRenderer.send('workspaceLayouts:save', { projectId, layout })
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
  saveScratchpadImage: (
    projectPath: string,
    notePath: string,
    bytes: Uint8Array,
    mime: string
  ): Promise<ScratchpadImageResult> =>
    ipcRenderer.invoke('biblio:saveScratchpadImage', projectPath, notePath, bytes, mime),
  readImageFile: (filePath: string): Promise<ImageFileResult> =>
    ipcRenderer.invoke('biblio:readImageFile', filePath),
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

const api = { pty, projects, workspaceLayouts, biblio, vault, windowControls }

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
