import { reactive, watch } from 'vue'
import { marked } from 'marked'
import * as tree from './layoutTree'
import type { DropZone, EdgeZone } from './layoutTree'
import { disposeShell, onShellExit, setShellArgs } from './shellRegistry'
import type { WorkspaceLayoutState, WorkspaceTab } from '../../preload/types'

export interface ProjectRef {
  id: string
  path: string
}

export interface DocEntry {
  editable: boolean
  content: string
  html: string
  status: 'loading' | 'ready' | 'missing'
  // Last content known to match disk, for editable docs -- lets flushDoc
  // tell an external write (another pane, a model) apart from its own echo.
  baseline: string
}

interface Workspace {
  layout: WorkspaceLayoutState
  docs: Record<string, DocEntry>
}

type DocTab = Extract<WorkspaceTab, { kind: 'doc' }>
type ShellTab = Extract<WorkspaceTab, { kind: 'shell' }>

const AUTOSAVE_MS = 600
const LAYOUT_SAVE_MS = 400
const SHELL_COMMAND = 'claude'

const workspaces = reactive<Record<string, Workspace>>({})
const projectRefs = new Map<string, ProjectRef>()
const initPromises = new Map<string, Promise<void>>()
const docSaveTimers = new Map<string, ReturnType<typeof setTimeout>>()
const layoutSaveTimers = new Map<string, ReturnType<typeof setTimeout>>()

export function workspaceOf(projectId: string): Workspace | undefined {
  return workspaces[projectId]
}

export function isEditablePath(path: string): boolean {
  return path.startsWith('scratchpad/')
}

export function projectRelativePath(path: string): string {
  return `biblio/${path}`
}

function newDocTab(path: string): DocTab {
  return { id: crypto.randomUUID(), kind: 'doc', path }
}

function tabsOf(projectId: string): WorkspaceTab[] {
  return tree.allLeaves(workspaces[projectId].layout.root).flatMap((leaf) => leaf.tabs)
}

function newShellTab(project: ProjectRef): ShellTab {
  const count = tabsOf(project.id).filter((tab) => tab.kind === 'shell').length
  return {
    id: crypto.randomUUID(),
    kind: 'shell',
    label: `${SHELL_COMMAND} ${count + 1}`,
    command: SHELL_COMMAND,
    cwd: project.path
  }
}

function openPaths(projectId: string): string[] {
  const paths = tabsOf(projectId).flatMap((tab) => (tab.kind === 'doc' ? [tab.path] : []))
  return [...new Set(paths)]
}

export function liveShellCount(projectId: string): number {
  return workspaces[projectId] ? tabsOf(projectId).filter((t) => t.kind === 'shell').length : 0
}

export function docTabCount(projectId: string): number {
  return workspaces[projectId] ? tabsOf(projectId).filter((t) => t.kind === 'doc').length : 0
}

async function loadDoc(project: ProjectRef, path: string, force = false): Promise<void> {
  const workspace = workspaces[project.id]
  const existing = workspace.docs[path]
  if (existing?.status === 'ready' && (!force || existing.editable)) return

  const editable = isEditablePath(path)
  if (!existing) {
    workspace.docs[path] = { editable, content: '', html: '', status: 'loading', baseline: '' }
  }
  const raw = await window.api.biblio.read(project.path, path)
  const entry = workspaces[project.id]?.docs[path]
  if (!entry) return
  if (raw === null) {
    entry.status = 'missing'
    return
  }
  if (editable) {
    entry.content = raw
    entry.baseline = raw
  } else entry.html = await marked.parse(raw)
  entry.status = 'ready'
}

async function flushDoc(project: ProjectRef, path: string): Promise<void> {
  const key = `${project.id}:${path}`
  const timer = docSaveTimers.get(key)
  if (!timer) return
  clearTimeout(timer)
  docSaveTimers.delete(key)
  const doc = workspaces[project.id]?.docs[path]
  if (!doc || doc.status !== 'ready') return

  // The file may have changed on disk since we last synced it -- another
  // pane or a model writing to the same scratchpad file. Saving our buffer
  // in that case would silently discard that write, so disk wins instead.
  const onDisk = await window.api.biblio.read(project.path, path)
  const current = workspaces[project.id]?.docs[path]
  if (!current) return
  if (onDisk !== null && onDisk !== current.baseline) {
    current.content = onDisk
    current.baseline = onDisk
    return
  }

  const ok = await window.api.biblio.write(project.path, path, current.content)
  if (ok) current.baseline = current.content
}

export function editDoc(project: ProjectRef, path: string, value: string): void {
  const doc = workspaces[project.id]?.docs[path]
  if (!doc) return
  doc.content = value
  const key = `${project.id}:${path}`
  clearTimeout(docSaveTimers.get(key))
  docSaveTimers.set(
    key,
    setTimeout(() => flushDoc(project, path), AUTOSAVE_MS)
  )
}

function scheduleLayoutSave(projectId: string): void {
  clearTimeout(layoutSaveTimers.get(projectId))
  layoutSaveTimers.set(
    projectId,
    setTimeout(() => {
      const layout = workspaces[projectId].layout
      window.api.workspaceLayouts.save(
        projectId,
        layout.root ? JSON.parse(JSON.stringify(layout)) : null
      )
    }, LAYOUT_SAVE_MS)
  )
}

function pruneDocs(project: ProjectRef): void {
  const workspace = workspaces[project.id]
  const used = new Set(openPaths(project.id))
  for (const path of Object.keys(workspace.docs)) {
    if (used.has(path)) continue
    flushDoc(project, path)
    delete workspace.docs[path]
  }
}

function commit(
  project: ProjectRef,
  root: tree.LayoutNode<WorkspaceTab> | null,
  focusPaneId?: string | null
): void {
  const layout = workspaces[project.id].layout
  layout.root = root
  if (focusPaneId !== undefined) layout.focusedPaneId = focusPaneId
  const leaves = tree.allLeaves(root)
  if (!leaves.some((leaf) => leaf.id === layout.focusedPaneId)) {
    layout.focusedPaneId = leaves[0]?.id ?? null
  }
  pruneDocs(project)
}

async function refreshDocs(project: ProjectRef): Promise<void> {
  const workspace = workspaces[project.id]
  const paths = new Set([...openPaths(project.id), ...Object.keys(workspace.docs)])
  await Promise.all([...paths].map((path) => loadDoc(project, path, true)))
}

async function init(project: ProjectRef): Promise<void> {
  const saved = await window.api.workspaceLayouts.load(project.id)
  workspaces[project.id] = {
    layout: { root: saved?.root ?? null, focusedPaneId: saved?.focusedPaneId ?? null },
    docs: {}
  }
  const layout = workspaces[project.id].layout
  const leaves = tree.allLeaves(layout.root)
  if (!leaves.some((leaf) => leaf.id === layout.focusedPaneId)) {
    layout.focusedPaneId = leaves[0]?.id ?? null
  }
  await Promise.all(openPaths(project.id).map((path) => loadDoc(project, path)))
  watch(
    () => workspaces[project.id].layout,
    () => scheduleLayoutSave(project.id),
    { deep: true }
  )
}

// First call loads the saved layout; later calls only refresh documents.
export function ensureLoaded(project: ProjectRef): Promise<void> {
  projectRefs.set(project.id, project)
  const existing = initPromises.get(project.id)
  if (existing) return existing.then(() => refreshDocs(project))
  const loading = init(project)
  initPromises.set(project.id, loading)
  return loading
}

export function refreshReadOnlyDocs(project: ProjectRef): Promise<void> {
  const initialized = initPromises.get(project.id)
  return initialized ? initialized.then(() => refreshDocs(project)) : Promise.resolve()
}

export function activePath(projectId: string): string | null {
  const layout = workspaces[projectId]?.layout
  if (!layout) return null
  const leaf = tree.findLeaf(layout.root, layout.focusedPaneId)
  const tab = leaf?.tabs.find((t) => t.id === leaf.activeTabId)
  return tab?.kind === 'doc' ? tab.path : null
}

export function openFile(project: ProjectRef, path: string): void {
  const layout = workspaces[project.id].layout
  const root = layout.root
  if (!root) {
    const leaf = tree.createLeaf<WorkspaceTab>([newDocTab(path)])
    commit(project, leaf, leaf.id)
    void loadDoc(project, path)
    return
  }
  const leaves = tree.allLeaves(root)
  const focused = leaves.find((leaf) => leaf.id === layout.focusedPaneId) ?? leaves[0]
  const hasPath = (leaf: (typeof leaves)[number]): boolean =>
    leaf.tabs.some((tab) => tab.kind === 'doc' && tab.path === path)
  const holder = hasPath(focused) ? focused : leaves.find(hasPath)
  if (holder) {
    const tab = holder.tabs.find((t) => t.kind === 'doc' && t.path === path)!
    commit(project, tree.setActiveTab(root, holder.id, tab.id), holder.id)
    return
  }
  commit(project, tree.addTab(root, focused.id, newDocTab(path)), focused.id)
  void loadDoc(project, path)
}

export function addShell(project: ProjectRef, paneId?: string): void {
  const layout = workspaces[project.id].layout
  const tab = newShellTab(project)
  const root = layout.root
  if (!root) {
    const leaf = tree.createLeaf<WorkspaceTab>([tab])
    commit(project, leaf, leaf.id)
    return
  }
  const leaves = tree.allLeaves(root)
  const target =
    leaves.find((leaf) => leaf.id === paneId) ??
    leaves.find((leaf) => leaf.id === layout.focusedPaneId) ??
    leaves[0]
  commit(project, tree.addTab(root, target.id, tab), target.id)
}

export function addShellInNewPane(project: ProjectRef, paneId: string, zone: EdgeZone): void {
  const root = workspaces[project.id].layout.root
  if (!root) return
  const tab = newShellTab(project)
  const next = tree.splitWithTab(root, paneId, zone, tab)
  commit(project, next, tree.findLeafByTab(next, tab.id)?.id)
}

export function openInNewShell(project: ProjectRef, paneId: string, tabId: string): void {
  const root = workspaces[project.id].layout.root
  const source = tree.findLeaf(root, paneId)?.tabs.find((tab) => tab.id === tabId)
  if (!root || source?.kind !== 'doc') return
  const tab = newShellTab(project)
  setShellArgs(tab.id, [
    `Read ${projectRelativePath(source.path)} and follow the instructions in it.`
  ])
  const next = tree.splitWithTab(root, paneId, 'right', tab)
  commit(project, next, tree.findLeafByTab(next, tab.id)?.id)
}

export function focusPane(project: ProjectRef, paneId: string): void {
  workspaces[project.id].layout.focusedPaneId = paneId
}

export function activateTab(project: ProjectRef, paneId: string, tabId: string): void {
  const root = workspaces[project.id].layout.root
  if (root) commit(project, tree.setActiveTab(root, paneId, tabId), paneId)
}

export function closeTab(project: ProjectRef, paneId: string, tabId: string): void {
  const root = workspaces[project.id].layout.root
  if (!root) return
  const tab = tree.findLeaf(root, paneId)?.tabs.find((t) => t.id === tabId)
  if (tab?.kind === 'shell') disposeShell(tab.id)
  commit(project, tree.closeTab(root, paneId, tabId))
}

export function resizeSplit(project: ProjectRef, splitId: string, sizes: number[]): void {
  const root = workspaces[project.id].layout.root
  if (root) workspaces[project.id].layout.root = tree.setSplitSizes(root, splitId, sizes)
}

export function placeTab(
  project: ProjectRef,
  fromPaneId: string,
  tabId: string,
  toPaneId: string,
  zone: DropZone,
  duplicate: boolean
): void {
  const root = workspaces[project.id].layout.root
  if (!root) return
  const source = tree.findLeaf(root, fromPaneId)?.tabs.find((tab) => tab.id === tabId)
  if (!source || (duplicate && source.kind === 'shell')) return
  const result = tree.placeTab(root, fromPaneId, tabId, toPaneId, zone, duplicate)
  if (!result) return
  commit(project, result.root, tree.findLeafByTab(result.root, result.placed.id)?.id)
}

function closeMatching(project: ProjectRef, matches: (tab: WorkspaceTab) => boolean): void {
  let root = workspaces[project.id].layout.root
  if (!root) return
  for (const leaf of tree.allLeaves(root)) {
    for (const tab of leaf.tabs) {
      if (!root || !matches(tab)) continue
      if (tab.kind === 'shell') disposeShell(tab.id)
      root = tree.closeTab(root, leaf.id, tab.id)
    }
  }
  commit(project, root)
}

export function closeUnder(project: ProjectRef, prefix: string): void {
  closeMatching(
    project,
    (tab) => tab.kind === 'doc' && (tab.path === prefix || tab.path.startsWith(`${prefix}/`))
  )
}

export function closeAllDocuments(project: ProjectRef): void {
  closeMatching(project, (tab) => tab.kind === 'doc')
}

export function closeAllTabs(project: ProjectRef): void {
  closeMatching(project, () => true)
}

onShellExit((tabId) => {
  for (const [projectId, project] of projectRefs) {
    const root = workspaces[projectId]?.layout.root
    const leaf = tree.findLeafByTab(root ?? null, tabId)
    if (root && leaf) {
      closeTab(project, leaf.id, tabId)
      return
    }
  }
})
