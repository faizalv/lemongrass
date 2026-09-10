<script setup lang="ts">
import type { Project } from '../../../preload'

defineProps<{
  projects: Project[]
  activeProjectId: string | null
  collapsed: boolean
}>()

const emit = defineEmits<{
  select: [id: string]
  add: []
  openDbChannels: []
}>()
</script>

<template>
  <div class="sidebar" :class="{ collapsed }">
    <div class="project-list">
      <button
        v-for="project in projects"
        :key="project.id"
        class="project-item"
        :class="{ active: project.id === activeProjectId }"
        :title="project.path"
        @click="emit('select', project.id)"
      >
        {{ project.name }}
      </button>
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

.project-item {
  text-align: left;
  padding: var(--space-2) var(--space-3);
  background: transparent;
  border: none;
  border-radius: var(--radius-lg);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: background var(--duration-fast) var(--ease-out);
}

.project-item:hover {
  background: var(--color-surface-2);
}

.project-item.active {
  background: var(--color-amber-muted);
  color: var(--color-fg-accent);
  font-weight: var(--weight-medium);
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
