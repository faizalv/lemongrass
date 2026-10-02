// Shared preload/renderer type contracts. Kept separate from index.ts, which
// imports 'electron' and other main-process-only packages the renderer's own
// tsconfig can't resolve -- the renderer and index.d.ts both import from here
// instead of dragging that implementation file into the web program.

export interface PtySpawnOptions {
  command: string
  args?: string[]
  cwd?: string
  tabId?: string
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

export interface VaultTokenPlacement {
  Kind: 'header' | 'cookie' | 'query'
  Name: string
  Prefix: string
}

export interface VaultDomainUser {
  Name: string
  Fields: Record<string, string>
  Token: string
  Tags: string[]
}

export interface VaultDomain {
  BaseURL: string
  LoginEndpoint: string
  TokenPath: string
  TTLOrigin: string
  FixedTTLSeconds: number
  TokenPlacement: VaultTokenPlacement
  Users: VaultDomainUser[]
}

export interface VaultMethodPath {
  Method: string
  PathPattern: string
}

export interface VaultHTTPScope {
  Methods: string[]
  Exclusions: VaultMethodPath[]
}

export interface VaultHTTPChannel {
  ID: string
  Name: string
  Domain: string
  Scope: VaultHTTPScope
  CreatedAt: string
  ExpiresAt: string
}

export interface VaultHTTPChannelWithShortId {
  channel: VaultHTTPChannel
  shortId: string
}

export interface VaultPolicy {
  rules: Record<string, 'allow' | 'deny' | 'approval'>
  binaries: string[]
  paths: string[]
}

export interface VaultPolicyRule {
  id: string
  category: string
  description: string
  mode: 'deny' | 'approval'
  locked: boolean
}

export type GitFileKind =
  | 'modified'
  | 'added'
  | 'deleted'
  | 'renamed'
  | 'copied'
  | 'typechange'
  | 'untracked'
  | 'conflicted'

export interface GitFile {
  path: string
  origPath?: string
  kind: GitFileKind
}

export interface GitStatus {
  isRepo: boolean
  branch: string | null
  detached: boolean
  upstream: string | null
  ahead: number
  behind: number
  hasCommits: boolean
  pushTarget: string | null
  files: GitFile[]
}

export interface GitDiffLine {
  number: number
  text: string
  type: 'context' | 'add' | 'del'
}

export interface GitDiffRow {
  left: GitDiffLine | null
  right: GitDiffLine | null
}

export interface GitDiffHunk {
  header: string
  rows: GitDiffRow[]
}

export type GitDiff =
  | { status: 'ok'; hunks: GitDiffHunk[]; added: number; removed: number }
  | { status: 'empty' }
  | { status: 'binary' }
  | { status: 'too-large' }
  | { status: 'error'; message: string }

export type GitActionResult = { ok: true } | { ok: false; error: string }

export type WorkspaceTab =
  | { id: string; kind: 'doc'; path: string }
  | { id: string; kind: 'diff'; path: string }
  | { id: string; kind: 'connector' }
  | { id: string; kind: 'workgroup'; groupId: number }
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

export interface WorkgroupSpawnMember {
  tabId: string
  label: string
  vendor: string
  model?: string
  prompt: string
}

export interface WorkgroupSpawnRequest {
  requestId: string
  projectPath: string
  pilotTabId: string
  members: WorkgroupSpawnMember[]
}

export interface WorkgroupSpawnResult {
  ok: boolean
  error?: string
}

export interface WorkgroupMemberInfo {
  tabId: string
  role: 'pilot' | 'copilot'
  label: string
  vendor: string
}

export interface WorkgroupInfo {
  id: number
  name: string
  threadId: number
  pilotTabId: string
  members: WorkgroupMemberInfo[]
}

export interface WorkgroupPendingMember {
  label: string
  vendor: string
  model?: string
  skills: string[]
  prompt: string
}

export interface WorkgroupPending {
  requestId: string
  projectPath: string
  groupName: string
  pilotLabel: string
  members: WorkgroupPendingMember[]
  createdAt: number
}

export interface WorkgroupThreadMessage {
  id: number
  tabId: string
  label: string
  body: string
  createdAt: string
}

export interface WorkgroupThread {
  id: number
  title: string
  groupId: number
  messages: WorkgroupThreadMessage[]
  more: boolean
}

export interface WorkgroupThreadResult {
  ok: boolean
  thread?: WorkgroupThread
  error?: string
}

export interface WorkgroupDisbandResult {
  ok: boolean
  error?: string
}
