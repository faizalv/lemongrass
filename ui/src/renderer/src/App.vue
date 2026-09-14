<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import ProjectSidebar from './components/ProjectSidebar.vue'
import HeaderBar from './components/HeaderBar.vue'
import PaneLayout from './components/PaneLayout.vue'
import BiblioManager from './components/BiblioManager.vue'
import DbChannelsPanel from './components/DbChannelsPanel.vue'
import {
  createTab,
  createLeaf,
  countTabs,
  firstLeafId,
  findLeaf,
  addTabToPane,
  setActiveTab,
  closeTab as closeTabInLayout,
  splitPane,
  setSplitSizes
} from './paneLayout'
import type { Project, PaneLayoutNode, BiblioTree } from '../../preload'

const projects = ref<Project[]>([])
const activeProjectId = ref<string | null>(null)

// Layout is scoped per project -- each project keeps its own split-pane
// tree, and switching projects switches which one shows. Persisted to
// ~/.lemongrass/layouts.json (see main/layouts.ts) so the arrangement
// survives a restart; each restored leaf spawns a fresh shell in place.
const layoutByProject = reactive<Record<string, PaneLayoutNode | null>>({})
const focusedPaneByProject = reactive<Record<string, string | null>>({})
const saveTimers: Record<string, ReturnType<typeof setTimeout>> = {}

// Live titles the agent CLI itself has set, keyed by tab id -- kept out of
// layoutByProject on purpose, same as the live PTY process and its
// scrollback: it's process state, not shape, so it never reaches
// layouts.json. A restored tab shows its static label again until its
// fresh shell sets a new one.
const titleByTab = reactive<Record<string, string>>({})

// Fetched on first visit and kept live afterward by onBiblioChanged, via the
// main-process watcher started on that first fetch. A biblio/ dir that
// doesn't exist yet on first visit still needs that project re-selected once
// it's been created, since nothing is watching the project root for it.
const biblioByProject = reactive<Record<string, BiblioTree | null>>({})
const mainView = ref<'workspace' | 'biblio'>('workspace')
const sidebarCollapsed = ref(false)
const showDbChannels = ref(false)

const activeProject = computed((): Project | undefined =>
  projects.value.find((p) => p.id === activeProjectId.value)
)
const currentLayout = computed((): PaneLayoutNode | null =>
  activeProjectId.value ? (layoutByProject[activeProjectId.value] ?? null) : null
)
const focusedPaneId = computed((): string | null =>
  activeProjectId.value ? (focusedPaneByProject[activeProjectId.value] ?? null) : null
)
const biblioTree = computed((): BiblioTree | null =>
  activeProjectId.value ? (biblioByProject[activeProjectId.value] ?? null) : null
)

async function loadProjects(): Promise<void> {
  projects.value = await window.api.projects.list()
  if (!activeProjectId.value && projects.value.length > 0) {
    await selectProject(projects.value[0].id)
  }
}

async function selectProject(id: string): Promise<void> {
  activeProjectId.value = id
  mainView.value = 'workspace'
  if (id in layoutByProject) return
  const loaded = await window.api.layouts.load(id)
  layoutByProject[id] = loaded
  focusedPaneByProject[id] = firstLeafId(loaded)

  const project = projects.value.find((p) => p.id === id)
  if (project) biblioByProject[id] = await window.api.biblio.tree(project.path)
}

async function refreshBiblio(): Promise<void> {
  const id = activeProjectId.value
  if (!id || !activeProject.value) return
  biblioByProject[id] = await window.api.biblio.tree(activeProject.value.path)
}

// Fires whenever anything adds/removes a file under a project's biblio/ --
// not only the app's own createScratchpad IPC call.
async function onBiblioChanged(projectPath: string): Promise<void> {
  const project = projects.value.find((p) => p.path === projectPath)
  if (!project) return
  biblioByProject[project.id] = await window.api.biblio.tree(projectPath)
}

async function addProject(): Promise<void> {
  const project = await window.api.projects.add()
  if (!project) return
  if (!projects.value.find((p) => p.id === project.id)) projects.value.push(project)
  await selectProject(project.id)
}

function saveLayout(projectId: string): void {
  clearTimeout(saveTimers[projectId])
  saveTimers[projectId] = setTimeout(() => {
    const layout = layoutByProject[projectId] ?? null
    // layoutByProject is reactive -- IPC's structured clone can't carry a
    // Vue proxy across the boundary, so this needs to cross as plain data.
    window.api.layouts.save(projectId, layout && JSON.parse(JSON.stringify(layout)))
  }, 400)
}

function withLayout(mutate: (layout: PaneLayoutNode) => PaneLayoutNode | null): void {
  const id = activeProjectId.value
  const layout = id ? layoutByProject[id] : undefined
  if (!id || !layout) return
  layoutByProject[id] = mutate(layout)
  saveLayout(id)
}

function newTab(): ReturnType<typeof createTab> | undefined {
  if (!activeProjectId.value || !activeProject.value) return undefined
  const count = countTabs(layoutByProject[activeProjectId.value] ?? null)
  return createTab('claude', activeProject.value.path, `claude ${count + 1}`)
}

function addTab(paneId?: string): void {
  const id = activeProjectId.value
  const tab = newTab()
  if (!id || !tab) return

  const layout = layoutByProject[id]
  const targetPane = paneId ?? focusedPaneByProject[id] ?? firstLeafId(layout) ?? undefined

  if (!layout) {
    const leaf = createLeaf([tab])
    layoutByProject[id] = leaf
    focusedPaneByProject[id] = leaf.id
  } else if (targetPane && findLeaf(layout, targetPane)) {
    layoutByProject[id] = addTabToPane(layout, targetPane, tab)
  } else {
    return
  }
  saveLayout(id)
}

function onFocus(paneId: string): void {
  if (activeProjectId.value) focusedPaneByProject[activeProjectId.value] = paneId
}

function onSelectTab(paneId: string, tabId: string): void {
  withLayout((layout) => setActiveTab(layout, paneId, tabId))
}

function onCloseTab(paneId: string, tabId: string): void {
  delete titleByTab[tabId]
  withLayout((layout) => closeTabInLayout(layout, paneId, tabId))
}

function onTitleChange(_paneId: string, tabId: string, title: string): void {
  titleByTab[tabId] = title
}

function onSplit(paneId: string, direction: 'row' | 'column'): void {
  const id = activeProjectId.value
  const tab = newTab()
  if (!id || !tab) return
  const newLeaf = createLeaf([tab])
  withLayout((layout) => splitPane(layout, paneId, direction, newLeaf))
  focusedPaneByProject[id] = newLeaf.id
}

function onResize(splitId: string, sizes: number[]): void {
  withLayout((layout) => setSplitSizes(layout, splitId, sizes))
}

function onExit(paneId: string, tabId: string): void {
  onCloseTab(paneId, tabId)
}

const ZOOM_STEP = 0.5
const ZOOM_MIN = -5
const ZOOM_MAX = 5
const zoomLevel = ref(0)

function setZoom(level: number): void {
  zoomLevel.value = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, level))
  window.electron.webFrame.setZoomLevel(zoomLevel.value)
}

// Captured, not bubbled -- xterm treats some Ctrl+key combos (Ctrl+- among
// them) as shell input and cancels the browser event on its own element
// before it would ever bubble up to a window-level listener.
function onKeydown(event: KeyboardEvent): void {
  if (!(event.ctrlKey || event.metaKey)) return
  if (event.key === '=' || event.key === '+') {
    event.preventDefault()
    event.stopPropagation()
    setZoom(zoomLevel.value + ZOOM_STEP)
  } else if (event.key === '-') {
    event.preventDefault()
    event.stopPropagation()
    setZoom(zoomLevel.value - ZOOM_STEP)
  } else if (event.key === '0') {
    event.preventDefault()
    event.stopPropagation()
    setZoom(0)
  }
}

let unsubscribeBiblio: (() => void) | undefined

onMounted(() => {
  loadProjects()
  window.addEventListener('keydown', onKeydown, { capture: true })
  unsubscribeBiblio = window.api.biblio.onChanged(onBiblioChanged)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown, { capture: true })
  unsubscribeBiblio?.()
})
</script>

<template>
  <div class="shell">
    <HeaderBar
      :sidebar-collapsed="sidebarCollapsed"
      @toggle-sidebar="sidebarCollapsed = !sidebarCollapsed"
    >
      <template #title>
        <span class="brand-mark">
          <svg
            width="12"
            height="12"
            viewBox="0 0 24 24"
            fill="none"
            stroke="var(--color-black)"
            stroke-width="2.4"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M11 20A7 7 0 0 1 9.8 6.1C15.5 5 17 4.48 19.2 2.2c1.7 6.6.2 13.1-5 17.7" />
            <path d="M2 21c0-3 1.85-5.36 5.08-6" />
          </svg>
        </span>
        <span class="wordmark">lemongrass</span>
      </template>
    </HeaderBar>

    <div class="body-row">
      <ProjectSidebar
        :projects="projects"
        :active-project-id="activeProjectId"
        :collapsed="sidebarCollapsed"
        :biblio-active="mainView === 'biblio'"
        @select="selectProject"
        @add="addProject"
        @open-db-channels="showDbChannels = true"
        @toggle-biblio="mainView = mainView === 'biblio' ? 'workspace' : 'biblio'"
      />

      <div v-if="activeProject" class="main">
        <div class="terminal-area">
          <template v-if="mainView === 'workspace'">
            <PaneLayout
              v-if="currentLayout"
              :node="currentLayout"
              :focused-pane-id="focusedPaneId"
              :titles="titleByTab"
              @focus="onFocus"
              @select-tab="onSelectTab"
              @close-tab="onCloseTab"
              @add-tab="addTab"
              @split="onSplit"
              @exit="onExit"
              @resize="onResize"
              @title-change="onTitleChange"
            />
            <div v-else class="empty-pane">
              <p class="empty">No shells open.</p>
              <button class="pill-button" @click="addTab()">+ New shell</button>
            </div>
          </template>
          <BiblioManager
            v-else-if="activeProject"
            :tree="biblioTree"
            :project-path="activeProject.path"
            @refresh="refreshBiblio"
          />
        </div>
      </div>

      <div v-else class="main empty-state">
        <p class="empty">Add a project to get started.</p>
      </div>
    </div>

    <DbChannelsPanel
      v-show="showDbChannels"
      :visible="showDbChannels"
      @close="showDbChannels = false"
    />
  </div>
</template>

<style scoped>
.shell {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--color-surface-0);
}

.brand-mark {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: var(--radius-sm);
  background: var(--color-amber);
  flex-shrink: 0;
  margin-right: var(--space-2);
}

.wordmark {
  font-family: var(--font-display);
  font-size: var(--text-md);
  font-weight: var(--weight-bold);
  color: var(--color-fg-accent);
  letter-spacing: var(--tracking-snug);
}

.pill-button {
  padding: var(--space-1) var(--space-3);
  background: transparent;
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-pill);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.pill-button:hover {
  color: var(--color-fg-primary);
}

.body-row {
  flex: 1;
  min-height: 0;
  display: flex;
}

.main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.main.empty-state {
  align-items: center;
  justify-content: center;
}

.terminal-area {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.empty-pane {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-3);
}

.empty {
  color: var(--color-fg-muted);
  font-size: var(--text-sm);
  padding: var(--space-4);
}
</style>
