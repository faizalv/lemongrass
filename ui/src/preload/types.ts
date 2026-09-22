// Shared preload/renderer type contracts. Kept separate from index.ts, which
// imports 'electron' and other main-process-only packages the renderer's own
// tsconfig can't resolve -- the renderer and index.d.ts both import from here
// instead of dragging that implementation file into the web program.

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

export type ScratchpadImageResult =
  { path: string } | { error: 'too-large' | 'unsupported' | 'failed' }

export type ImageFileResult =
  { bytes: Uint8Array; mime: string } | { error: 'too-large' | 'unsupported' | 'failed' }

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

export type WorkspaceTab =
  | { id: string; kind: 'doc'; path: string }
  | { id: string; kind: 'shell'; label: string; command: string; cwd?: string }

export type WorkspaceLayoutNode =
  | { type: 'leaf'; id: string; tabs: WorkspaceTab[]; activeTabId: string | null }
  | {
      type: 'split'
      id: string
      direction: 'row' | 'column'
      children: WorkspaceLayoutNode[]
      sizes: number[]
    }

export interface WorkspaceLayoutState {
  root: WorkspaceLayoutNode | null
  focusedPaneId: string | null
}
