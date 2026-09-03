<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import ProjectSidebar from './components/ProjectSidebar.vue'
import HeaderBar from './components/HeaderBar.vue'
import PaneLayout from './components/PaneLayout.vue'
import {
  createTab,
  createLeaf,
  countTabs,
  firstLeafId,
  findLeaf,
  addTabToPane,
  setActiveTab,
  closeTab as closeTabInLayout,
  closePane as closePaneInLayout,
  splitPane,
  setSplitSizes
} from './paneLayout'
import type { Project, PaneLayoutNode } from '../../preload'

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

const activeProject = computed((): Project | undefined =>
  projects.value.find((p) => p.id === activeProjectId.value)
)
const currentLayout = computed((): PaneLayoutNode | null =>
  activeProjectId.value ? (layoutByProject[activeProjectId.value] ?? null) : null
)
const focusedPaneId = computed((): string | null =>
  activeProjectId.value ? (focusedPaneByProject[activeProjectId.value] ?? null) : null
)

async function loadProjects(): Promise<void> {
  projects.value = await window.api.projects.list()
  if (!activeProjectId.value && projects.value.length > 0) {
    await selectProject(projects.value[0].id)
  }
}

async function selectProject(id: string): Promise<void> {
  activeProjectId.value = id
  if (id in layoutByProject) return
  const loaded = await window.api.layouts.load(id)
  layoutByProject[id] = loaded
  focusedPaneByProject[id] = firstLeafId(loaded)
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

function onClosePane(paneId: string): void {
  const id = activeProjectId.value
  const layout = id ? (layoutByProject[id] ?? null) : null
  for (const tab of findLeaf(layout, paneId)?.tabs ?? []) delete titleByTab[tab.id]
  withLayout((layout) => closePaneInLayout(layout, paneId))
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

onMounted(loadProjects)
</script>

<template>
  <div class="shell">
    <HeaderBar>
      <template #title>
        <span class="app-title">{{ activeProject?.name ?? 'lemongrass' }}</span>
      </template>
    </HeaderBar>

    <div class="body-row">
      <ProjectSidebar
        :projects="projects"
        :active-project-id="activeProjectId"
        @select="selectProject"
        @add="addProject"
      />

      <div v-if="activeProject" class="main">
        <div class="terminal-area">
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
            @close-pane="onClosePane"
            @exit="onExit"
            @resize="onResize"
            @title-change="onTitleChange"
          />
          <div v-else class="empty-pane">
            <p class="empty">No shells open.</p>
            <button class="pill-button" @click="addTab()">+ New shell</button>
          </div>
        </div>
      </div>

      <div v-else class="main empty-state">
        <p class="empty">Add a project to get started.</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.shell {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--color-surface-0);
}

.app-title {
  font-family: var(--font-display);
  font-size: var(--text-sm);
  font-weight: var(--weight-semibold);
  color: var(--color-fg-primary);
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
