import { ElectronAPI } from '@electron-toolkit/preload'
import type { PtySpawnOptions, PtyDataPayload, PtyExitPayload, Project, TreeNode } from './index'

interface Api {
  pty: {
    spawn: (opts: PtySpawnOptions) => Promise<{ id: string }>
    write: (id: string, data: string) => void
    resize: (id: string, cols: number, rows: number) => void
    kill: (id: string) => void
    onData: (callback: (payload: PtyDataPayload) => void) => () => void
    onExit: (callback: (payload: PtyExitPayload) => void) => () => void
  }
  projects: {
    list: () => Promise<Project[]>
    add: () => Promise<Project | null>
  }
  knowledge: {
    tree: (projectPath: string) => Promise<TreeNode[]>
    read: (projectPath: string, filePath: string) => Promise<string>
  }
  windowControls: {
    minimize: () => void
    toggleMaximize: () => void
    close: () => void
    isMaximized: () => Promise<boolean>
    onMaximizedChange: (callback: (maximized: boolean) => void) => () => void
  }
}

declare global {
  interface Window {
    electron: ElectronAPI
    api: Api
  }
}
