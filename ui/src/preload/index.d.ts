import { ElectronAPI } from '@electron-toolkit/preload'
import type {
  PtySpawnOptions,
  PtyDataPayload,
  PtyExitPayload,
  Project,
  PaneLayoutNode,
  BiblioTree
} from './index'

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
  layouts: {
    load: (projectId: string) => Promise<PaneLayoutNode | null>
    save: (projectId: string, layout: PaneLayoutNode | null) => void
  }
  biblio: {
    tree: (projectPath: string) => Promise<BiblioTree | null>
    read: (projectPath: string, relativePath: string) => Promise<string | null>
    write: (projectPath: string, relativePath: string, content: string) => Promise<boolean>
    createScratchpad: (
      projectPath: string,
      title: string,
      content: string
    ) => Promise<string | null>
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
