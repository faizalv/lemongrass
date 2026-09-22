<script setup lang="ts">
import type { Project } from '../../../preload/types'

defineProps<{
  projects: Project[]
  activeProjectId: string | null
  collapsed: boolean
  biblioActive: boolean
}>()

const emit = defineEmits<{
  select: [id: string]
  add: []
  openDbChannels: []
  toggleBiblio: []
}>()
</script>

<template>
  <div class="sidebar" :class="{ collapsed }">
    <div class="project-list">
      <div
        v-for="project in projects"
        :key="project.id"
        class="project-row"
        :class="{ active: project.id === activeProjectId }"
      >
        <button class="project-item" :title="project.path" @click="emit('select', project.id)">
          {{ project.name }}
        </button>
        <button
          v-if="project.id === activeProjectId"
          class="biblio-toggle"
          :class="{ active: biblioActive }"
          title="Biblio"
          @click="emit('toggleBiblio')"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 14 14"
            fill="none"
            stroke="currentColor"
            stroke-width="1.1"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M7 3.2C5.8 2.3 3.8 2 1.8 2.4v8.3c2-.4 4 0 5.2.9" />
            <path d="M7 3.2C8.2 2.3 10.2 2 12.2 2.4v8.3c-2-.4-4 0-5.2.9" />
          </svg>
        </button>
      </div>
      <p v-if="projects.length === 0" class="empty">No projects yet.</p>
    </div>
    <button class="add-project" @click="emit('add')">
      <span class="icon">+</span>
      Add project
    </button>
    <button class="add-project" @click="emit('openDbChannels')">
      <span class="icon">⛁</span>
      Database access
    </button>
  </div>
</template>

<style scoped>
.sidebar {
  width: 220px;
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

.project-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--space-3) var(--space-3) var(--space-1);
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.project-row {
  display: flex;
  align-items: center;
  border-radius: var(--radius-lg);
  transition: background var(--duration-fast) var(--ease-out);
}

.project-row:hover {
  background: var(--color-surface-2);
}

.project-row.active {
  background: var(--color-amber-muted);
}

.project-item {
  flex: 1;
  min-width: 0;
  text-align: left;
  padding: var(--space-2) var(--space-3);
  background: transparent;
  border: none;
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.project-row.active .project-item {
  color: var(--color-fg-accent);
  font-weight: var(--weight-medium);
}

.biblio-toggle {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  margin-right: var(--space-2);
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  border-radius: var(--radius-pill);
  color: var(--color-fg-accent);
  opacity: 0.6;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    opacity var(--duration-fast) var(--ease-out);
}

.biblio-toggle:hover {
  background: var(--color-surface-3);
  opacity: 1;
}

.biblio-toggle.active {
  background: var(--color-amber);
  color: var(--color-black);
  opacity: 1;
}

.empty {
  padding: var(--space-2) var(--space-3);
  color: var(--color-fg-muted);
  font-size: var(--text-sm);
}

.add-project {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  box-sizing: border-box;
  padding: var(--space-2) var(--space-3);
  background: transparent;
  border: none;
  border-radius: 0;
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  text-align: left;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.add-project:hover {
  background: var(--color-amber);
  color: var(--color-black);
}

.icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: var(--text-sm);
  line-height: 1;
}
</style>
