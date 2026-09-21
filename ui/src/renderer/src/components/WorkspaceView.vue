<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import WorkspaceLayout from './WorkspaceLayout.vue'
import {
  addShell,
  closeAllDocuments,
  closeAllTabs,
  docTabCount,
  liveShellCount,
  workspaceOf,
  type ProjectRef
} from '../workspace'

const props = defineProps<{
  project: ProjectRef
}>()

const workspace = computed(() => workspaceOf(props.project.id))
const shellCount = computed(() => liveShellCount(props.project.id))
const documentCount = computed(() => docTabCount(props.project.id))

const closeAllSummary = computed((): string => {
  const shells = `${shellCount.value} running ${shellCount.value === 1 ? 'shell' : 'shells'}`
  const documents = `${documentCount.value} ${documentCount.value === 1 ? 'document' : 'documents'}`
  if (shellCount.value && documentCount.value) return `This ends ${shells} and closes ${documents}.`
  if (shellCount.value) return `This ends ${shells}.`
  return `This closes ${documents}.`
})

const confirmingDocuments = ref(false)
const confirmingAll = ref(false)
let confirmTimer: ReturnType<typeof setTimeout> | undefined

function onCloseDocuments(): void {
  if (!confirmingDocuments.value) {
    confirmingDocuments.value = true
    confirmTimer = setTimeout(() => (confirmingDocuments.value = false), 3000)
    return
  }
  clearTimeout(confirmTimer)
  confirmingDocuments.value = false
  closeAllDocuments(props.project)
}

function onCloseAllTabs(): void {
  closeAllTabs(props.project)
  confirmingAll.value = false
}

onBeforeUnmount(() => clearTimeout(confirmTimer))
</script>

<template>
  <div class="workspace-view">
    <template v-if="workspace?.layout.root">
      <div class="toolbar">
        <button class="toolbar-button" @click="addShell(project)">New shell</button>
        <button class="toolbar-button" :disabled="!documentCount" @click="onCloseDocuments">
          {{ confirmingDocuments ? 'Click again to close all documents' : 'Close all documents' }}
        </button>
        <button class="toolbar-button" @click="confirmingAll = true">Close all tabs</button>
      </div>
      <div class="layout-area">
        <WorkspaceLayout
          :node="workspace.layout.root"
          :project="project"
          :focused-pane-id="workspace.layout.focusedPaneId"
        />
      </div>
    </template>
    <div v-else class="empty-pane">
      <p class="empty">Nothing open.</p>
      <button class="pill-button" @click="addShell(project)">+ New shell</button>
    </div>

    <div v-if="confirmingAll" class="overlay">
      <div class="card">
        <h3 class="card-title">Close all tabs?</h3>
        <p class="card-text">{{ closeAllSummary }}</p>
        <div class="card-actions">
          <button class="ghost-button" @click="confirmingAll = false">Cancel</button>
          <button class="primary-button" @click="onCloseAllTabs">Close all tabs</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.workspace-view {
  position: relative;
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.toolbar {
  flex-shrink: 0;
  display: flex;
  justify-content: flex-end;
  gap: var(--space-1);
  padding: var(--space-2) var(--space-4) 0;
}

.toolbar-button {
  padding: var(--space-1) var(--space-3);
  background: transparent;
  border: none;
  border-radius: var(--radius-pill);
  color: var(--color-fg-muted);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.toolbar-button:hover:not(:disabled) {
  background: var(--color-surface-1);
  color: var(--color-fg-primary);
}

.toolbar-button:disabled {
  opacity: 0.5;
  cursor: default;
}

.layout-area {
  flex: 1;
  min-height: 0;
  display: flex;
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

.overlay {
  position: absolute;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-8);
  background: var(--color-bg-overlay);
}

.card {
  width: 100%;
  max-width: 420px;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-6);
  background: var(--color-surface-1);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-xl);
}

.card-title {
  font-family: var(--font-display);
  font-size: var(--text-md);
  font-weight: var(--weight-semibold);
  color: var(--color-fg-primary);
}

.card-text {
  color: var(--color-fg-secondary);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}

.card-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}

.ghost-button,
.primary-button {
  padding: var(--space-1) var(--space-4);
  border-radius: var(--radius-pill);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.ghost-button {
  background: transparent;
  border: 1px solid var(--color-border-default);
  color: var(--color-fg-secondary);
}

.ghost-button:hover {
  color: var(--color-fg-primary);
  background: var(--color-surface-2);
}

.primary-button {
  background: var(--color-amber);
  border: none;
  color: var(--color-black);
}

.primary-button:hover {
  background: var(--color-amber-dim);
}
</style>
