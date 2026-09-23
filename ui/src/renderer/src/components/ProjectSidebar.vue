<script setup lang="ts">
import BiblioPanel from './BiblioPanel.vue'
import type { BiblioTree } from '../../../preload/types'
import type { ProjectRef } from '../workspace'

defineProps<{
  collapsed: boolean
  project: ProjectRef | undefined
  tree: BiblioTree | null
}>()

const emit = defineEmits<{
  openConnector: []
  refreshBiblio: []
}>()
</script>

<template>
  <div class="sidebar" :class="{ collapsed }">
    <div class="sidebar-main">
      <BiblioPanel
        v-if="project"
        :tree="tree"
        :project="project"
        @refresh="emit('refreshBiblio')"
      />
      <p v-else class="empty">No project selected.</p>
    </div>
    <button class="connector-button" @click="emit('openConnector')">
      <span class="icon-col">
        <svg
          class="connector-icon"
          width="14"
          height="14"
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <line x1="4.5" y1="3.5" x2="4.5" y2="8.5" />
          <line x1="7" y1="3.5" x2="7" y2="8.5" />
          <line x1="9.5" y1="3.5" x2="9.5" y2="8.5" />
          <rect x="2.5" y="8.5" width="9" height="2.5" rx="0.6" />
        </svg>
      </span>
      <span class="label">Connector</span>
    </button>
  </div>
</template>

<style scoped>
.sidebar {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--color-surface-1);
  overflow: hidden;
  transition: width var(--duration-base) var(--ease-out);
}

.sidebar.collapsed {
  width: 0;
}

.sidebar-main {
  flex: 1;
  min-height: 0;
  display: flex;
}

.empty {
  padding: var(--space-4) var(--space-3);
  color: var(--color-fg-muted);
  font-size: var(--text-sm);
}

.connector-button {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  box-sizing: border-box;
  padding: var(--space-2) var(--space-3);
  background: transparent;
  border: none;
  border-top: 1px solid var(--color-border-default);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  text-align: left;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.connector-button:hover {
  background: var(--color-amber);
  color: var(--color-black);
}

.icon-col {
  flex-shrink: 0;
  width: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.connector-icon {
  flex-shrink: 0;
}

.label {
  flex: 1;
  text-align: left;
}
</style>
