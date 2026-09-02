<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import TerminalPane from './components/TerminalPane.vue'
import ProjectSidebar from './components/ProjectSidebar.vue'
import KnowledgePanel from './components/KnowledgePanel.vue'
import HeaderBar from './components/HeaderBar.vue'
import type { Project } from '../../preload'

interface Tab {
  id: number
  label: string
}

const projects = ref<Project[]>([])
const activeProjectId = ref<string | null>(null)
const showKnowledge = ref(false)

// Shells are scoped per project -- each project keeps its own tab set,
// and switching projects switches which shells show.
const tabsByProject = reactive<Record<string, Tab[]>>({})
const activeTabByProject = reactive<Record<string, number>>({})
let nextTabId = 1

const activeProject = computed((): Project | undefined =>
  projects.value.find((p) => p.id === activeProjectId.value)
)
const currentTabs = computed((): Tab[] =>
  activeProjectId.value ? (tabsByProject[activeProjectId.value] ?? []) : []
)

async function loadProjects(): Promise<void> {
  projects.value = await window.api.projects.list()
  if (!activeProjectId.value && projects.value.length > 0) {
    selectProject(projects.value[0].id)
  }
}

function selectProject(id: string): void {
  activeProjectId.value = id
  if (!tabsByProject[id]) tabsByProject[id] = []
}

async function addProject(): Promise<void> {
  const project = await window.api.projects.add()
  if (!project) return
  if (!projects.value.find((p) => p.id === project.id)) projects.value.push(project)
  selectProject(project.id)
}

function addTab(): void {
  const id = activeProjectId.value
  if (!id) return
  const tab = { id: nextTabId++, label: `claude ${nextTabId - 1}` }
  tabsByProject[id].push(tab)
  activeTabByProject[id] = tab.id
}

function closeTab(tabId: number): void {
  const id = activeProjectId.value
  if (!id) return
  const tabs = tabsByProject[id]
  const idx = tabs.findIndex((t) => t.id === tabId)
  if (idx === -1) return
  tabs.splice(idx, 1)
  if (activeTabByProject[id] === tabId) {
    activeTabByProject[id] = tabs[Math.max(0, idx - 1)]?.id
  }
}

onMounted(loadProjects)
</script>

<template>
  <div class="shell">
    <HeaderBar>
      <template #title>
        <span class="app-title">{{ activeProject?.name ?? 'lemongrass' }}</span>
      </template>
      <template #actions>
        <button
          v-if="activeProject"
          class="knowledge-toggle"
          :class="{ active: showKnowledge }"
          @click="showKnowledge = !showKnowledge"
        >
          Knowledge
        </button>
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
          <div class="tab-bar">
            <button
              v-for="tab in currentTabs"
              :key="tab.id"
              class="tab"
              :class="{ active: tab.id === activeTabByProject[activeProject.id] }"
              @click="activeTabByProject[activeProject.id] = tab.id"
            >
              {{ tab.label }}
              <span class="tab-close" @click.stop="closeTab(tab.id)">&times;</span>
            </button>
            <button class="tab-add" @click="addTab">+</button>
          </div>
          <div class="panes">
            <p v-if="currentTabs.length === 0" class="empty">
              No shells open. Click + to start one.
            </p>
            <TerminalPane
              v-for="tab in currentTabs"
              v-show="tab.id === activeTabByProject[activeProject.id]"
              :key="`${activeProject.id}-${tab.id}`"
              command="claude"
              :cwd="activeProject.path"
              @exit="closeTab(tab.id)"
            />
          </div>
        </div>

        <KnowledgePanel v-if="showKnowledge" :project-path="activeProject.path" />
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

.knowledge-toggle {
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

.knowledge-toggle:hover {
  color: var(--color-fg-primary);
}

.knowledge-toggle.active {
  background: var(--color-amber-muted);
  color: var(--color-fg-accent);
  border-color: transparent;
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

.tab-bar {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4) var(--space-2);
}

.tab {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-4);
  background: transparent;
  border: none;
  border-radius: var(--radius-pill);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-out);
}

.tab:hover {
  background: var(--color-surface-1);
}

.tab.active {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.tab-close {
  color: var(--color-fg-muted);
  border-radius: var(--radius-pill);
}

.tab-close:hover {
  color: var(--color-fg-primary);
}

.tab-add {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  background: transparent;
  border: none;
  border-radius: var(--radius-pill);
  color: var(--color-fg-accent);
  font-size: var(--text-md);
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-out);
}

.tab-add:hover {
  background: var(--color-surface-1);
}

.panes {
  flex: 1;
  min-height: 0;
  position: relative;
  padding: 0 var(--space-4) var(--space-4);
}

.panes > .terminal-card {
  height: 100%;
}

.empty {
  color: var(--color-fg-muted);
  font-size: var(--text-sm);
  padding: var(--space-4);
}
</style>
