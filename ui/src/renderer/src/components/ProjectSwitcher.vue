<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { Project } from '../../../preload/types'

const props = defineProps<{
  projects: Project[]
  activeProjectId: string | null
}>()

const emit = defineEmits<{
  select: [id: string]
  add: []
}>()

const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)

const activeProject = computed((): Project | undefined =>
  props.projects.find((p) => p.id === props.activeProjectId)
)

function toggleOpen(): void {
  open.value = !open.value
}

function pick(id: string): void {
  emit('select', id)
  open.value = false
}

function pickAdd(): void {
  emit('add')
  open.value = false
}

function onDocumentClickOutside(event: MouseEvent): void {
  if (!open.value) return
  if (rootRef.value && !rootRef.value.contains(event.target as Node)) {
    open.value = false
  }
}

onMounted(() => document.addEventListener('click', onDocumentClickOutside, true))
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClickOutside, true))
</script>

<template>
  <div ref="rootRef" class="switcher">
    <button class="trigger" @click="toggleOpen">
      <span class="trigger-label">{{ activeProject?.name ?? 'No project' }}</span>
      <svg
        class="caret"
        :class="{ open }"
        width="9"
        height="9"
        viewBox="0 0 10 10"
        fill="none"
        stroke="currentColor"
        stroke-width="1.4"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M2 3.5L5 6.5L8 3.5" />
      </svg>
    </button>

    <div v-if="open" class="dropdown">
      <div class="project-list">
        <button
          v-for="project in projects"
          :key="project.id"
          class="project-row"
          :class="{ active: project.id === activeProjectId }"
          :title="project.path"
          @click="pick(project.id)"
        >
          {{ project.name }}
        </button>
        <p v-if="projects.length === 0" class="empty">No projects yet.</p>
      </div>
      <button class="add-project" @click="pickAdd">
        <span class="icon">+</span>
        <span>Add project</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.switcher {
  position: relative;
}

.trigger {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-1) var(--space-2);
  background: transparent;
  border: none;
  border-radius: var(--radius-md);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-out);
}

.trigger:hover {
  background: var(--color-surface-2);
}

.trigger-label {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.caret {
  flex-shrink: 0;
  transition: transform var(--duration-fast) var(--ease-out);
}

.caret.open {
  transform: rotate(180deg);
}

.dropdown {
  position: absolute;
  top: calc(100% + var(--space-2));
  left: 0;
  z-index: 80;
  width: 260px;
  display: flex;
  flex-direction: column;
  background: var(--color-surface-1);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-xl);
  overflow: hidden;
}

.project-list {
  max-height: 320px;
  overflow-y: auto;
  padding: var(--space-2);
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.project-row {
  text-align: left;
  padding: var(--space-2) var(--space-3);
  background: transparent;
  border: none;
  border-radius: var(--radius-md);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: background var(--duration-fast) var(--ease-out);
}

.project-row:hover {
  background: var(--color-surface-2);
}

.project-row.active {
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

.add-project:hover {
  background: var(--color-amber);
  color: var(--color-black);
}

.icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  font-size: var(--text-sm);
  line-height: 1;
}
</style>
