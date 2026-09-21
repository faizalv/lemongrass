import { reactive, watch } from 'vue'
import { marked } from 'marked'
import * as tree from './layoutTree'
import type { DropZone } from './layoutTree'
import type { DocLayoutState, DocTab } from '../../preload'

export interface ProjectRef {
  id: string
  path: string
}

export interface DocEntry {
  editable: boolean
  content: string
  html: string
  status: 'loading' | 'ready' | 'missing'
}

interface Workspace {
  layout: DocLayoutState
  docs: Record<string, DocEntry>
}

const AUTOSAVE_MS = 600
const LAYOUT_SAVE_MS = 400

const workspaces = reactive<Record<string, Workspace>>({})
const initPromises = new Map<string, Promise<void>>()
const docSaveTimers = new Map<string, ReturnType<typeof setTimeout>>()
const layoutSaveTimers = new Map<string, ReturnType<typeof setTimeout>>()

export function workspaceOf(projectId: string): Workspace | undefined {
  return workspaces[projectId]
}

export function isEditablePath(path: string): boolean {
  return path.startsWith('scratchpad/')
}

function newTab(path: string): DocTab {
  return { id: crypto.randomUUID(), path }
}

function openPaths(projectId: string): string[] {
  const leaves = tree.allLeaves(workspaces[projectId].layout.root)
  return [...new Set(leaves.flatMap((leaf) => leaf.tabs.map((tab) => tab.path)))]
}

async function loadDoc(project: ProjectRef, path: string, force = false): Promise<void> {
  const workspace = workspaces[project.id]
  const existing = workspace.docs[path]
  if (existing?.status === 'ready' && (!force || existing.editable)) return

  const editable = isEditablePath(path)
  if (!existing) {
    workspace.docs[path] = { editable, content: '', html: '', status: 'loading' }
  }
  const raw = await window.api.biblio.read(project.path, path)
  const entry = workspaces[project.id]?.docs[path]
  if (!entry) return
  if (raw === null) {
    entry.status = 'missing'
    return
  }
  if (editable) entry.content = raw
  else entry.html = await marked.parse(raw)
  entry.status = 'ready'
}

function flushDoc(project: ProjectRef, path: string): void {
  const key = `${project.id}:${path}`
  const timer = docSaveTimers.get(key)
  if (!timer) return
  clearTimeout(timer)
  docSaveTimers.delete(key)
  const doc = workspaces[project.id]?.docs[path]
  if (doc && doc.status === 'ready') void window.api.biblio.write(project.path, path, doc.content)
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
      window.api.docLayouts.save(projectId, layout.root ? JSON.parse(JSON.stringify(layout)) : null)
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
  root: tree.LayoutNode<DocTab> | null,
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
  const saved = await window.api.docLayouts.load(project.id)
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
  return leaf?.tabs.find((tab) => tab.id === leaf.activeTabId)?.path ?? null
}

export function openFile(project: ProjectRef, path: string): void {
  const layout = workspaces[project.id].layout
  const root = layout.root
  if (!root) {
    const leaf = tree.createLeaf([newTab(path)])
    commit(project, leaf, leaf.id)
    void loadDoc(project, path)
    return
  }
  const leaves = tree.allLeaves(root)
  const focused = leaves.find((leaf) => leaf.id === layout.focusedPaneId) ?? leaves[0]
  const holder = focused.tabs.some((tab) => tab.path === path)
    ? focused
    : leaves.find((leaf) => leaf.tabs.some((tab) => tab.path === path))
  if (holder) {
    const tab = holder.tabs.find((t) => t.path === path)!
    commit(project, tree.setActiveTab(root, holder.id, tab.id), holder.id)
    return
  }
  commit(project, tree.addTab(root, focused.id, newTab(path)), focused.id)
  void loadDoc(project, path)
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
  if (root) commit(project, tree.closeTab(root, paneId, tabId))
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
  const result = tree.placeTab(root, fromPaneId, tabId, toPaneId, zone, duplicate)
  if (!result) return
  commit(project, result.root, tree.findLeafByTab(result.root, result.placed.id)?.id)
}

export function closeUnder(project: ProjectRef, prefix: string): void {
  let root = workspaces[project.id].layout.root
  if (!root) return
  const matches = (path: string): boolean => path === prefix || path.startsWith(`${prefix}/`)
  for (const leaf of tree.allLeaves(root)) {
    for (const tab of leaf.tabs) {
      if (root && matches(tab.path)) root = tree.closeTab(root, leaf.id, tab.id)
    }
  }
  commit(project, root)
}

export function closeAll(project: ProjectRef): void {
  commit(project, null, null)
}
