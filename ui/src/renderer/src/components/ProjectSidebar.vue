<script setup lang="ts">
import type { Project } from '../../../preload'

defineProps<{
  projects: Project[]
  activeProjectId: string | null
}>()

const emit = defineEmits<{
  select: [id: string]
  add: []
}>()
</script>

<template>
  <div class="sidebar">
    <div class="sidebar-header">lemongrass</div>
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
      <span class="plus">+</span>
      Add project
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
}

.sidebar-header {
  padding: var(--space-5) var(--space-4) var(--space-3);
  font-family: var(--font-display);
  font-size: var(--text-md);
  font-weight: var(--weight-bold);
  color: var(--color-fg-accent);
  letter-spacing: var(--tracking-snug);
}

.project-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--space-1) var(--space-3);
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
  margin: var(--space-3);
  padding: var(--space-2) var(--space-3);
  background: transparent;
  border: none;
  border-radius: var(--radius-pill);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-out);
}

.add-project:hover {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.plus {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: var(--radius-pill);
  background: var(--color-surface-3);
  font-size: var(--text-xs);
  line-height: 1;
}
</style>
