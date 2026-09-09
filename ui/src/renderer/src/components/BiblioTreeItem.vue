<script setup lang="ts">
import { computed, ref } from 'vue'
import type { BiblioNode } from '../../../preload'

const props = defineProps<{
  node: BiblioNode
  selectedPath: string | null
  depth: number
}>()

const emit = defineEmits<{
  select: [path: string]
}>()

const expanded = ref(false)

// Only the four top-level category dirs get a distinct icon -- anything
// nested (a book's own folder, handover/archive) reads as a plain folder.
const CATEGORY_ICONS = new Set(['books', 'handover', 'laws', 'scratchpad'])

const iconName = computed((): string => {
  if (props.node.type === 'file') return 'file'
  if (props.depth === 0 && CATEGORY_ICONS.has(props.node.name)) return props.node.name
  return 'folder'
})
</script>

<template>
  <button
    v-if="node.type === 'file'"
    class="tree-item file"
    :class="{ active: node.path === selectedPath }"
    :style="{ paddingLeft: `${depth * 14 + 10}px` }"
    @click="emit('select', node.path)"
  >
    <span class="indent" />
    <svg
      class="icon"
      width="13"
      height="13"
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      stroke-width="1.2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M3 1.5h5l3 3v8H3z" />
      <path d="M8 1.5v3h3" />
    </svg>
    <span class="label">{{ node.name.replace(/\.md$/, '') }}</span>
  </button>

  <button
    v-else
    class="tree-item dir"
    :style="{ paddingLeft: `${depth * 14 + 10}px` }"
    @click="expanded = !expanded"
  >
    <svg
      class="indent chevron"
      :class="{ expanded }"
      width="9"
      height="9"
      viewBox="0 0 9 9"
      fill="none"
      stroke="currentColor"
      stroke-width="1.4"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M2 1.5 6.5 4.5 2 7.5" />
    </svg>

    <svg
      v-if="iconName === 'books'"
      class="icon"
      width="13"
      height="13"
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      stroke-width="1.2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <rect x="2" y="1.5" width="10" height="11" rx="1" />
      <line x1="5" y1="1.5" x2="5" y2="12.5" />
    </svg>
    <svg
      v-else-if="iconName === 'laws'"
      class="icon"
      width="13"
      height="13"
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      stroke-width="1.2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M7 1 12 3 12 7 7 13 2 7 2 3 Z" />
    </svg>
    <svg
      v-else-if="iconName === 'handover'"
      class="icon"
      width="13"
      height="13"
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      stroke-width="1.2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M1.5 4.5h9M8 2.5l2.5 2-2.5 2" />
      <path d="M12.5 9.5h-9M6 11.5l-2.5-2 2.5-2" />
    </svg>
    <svg
      v-else-if="iconName === 'scratchpad'"
      class="icon"
      width="13"
      height="13"
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      stroke-width="1.2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M9.5 1.5 12.5 4.5 5 12H2V9z" />
    </svg>
    <svg
      v-else
      class="icon"
      width="13"
      height="13"
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      stroke-width="1.2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M1.5 3.5h4l1.5 2h5.5v6h-11z" />
    </svg>

    <span class="label">{{ node.name }}</span>
  </button>

  <template v-if="node.children && expanded">
    <BiblioTreeItem
      v-for="child in node.children"
      :key="child.path"
      :node="child"
      :selected-path="selectedPath"
      :depth="depth + 1"
      @select="(path) => emit('select', path)"
    />
  </template>
</template>

<style scoped>
.tree-item {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-width: 0;
  text-align: left;
  padding-top: var(--space-1);
  padding-bottom: var(--space-1);
  padding-right: var(--space-3);
  background: transparent;
  border: none;
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  cursor: pointer;
  border-radius: var(--radius-md);
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.tree-item:hover {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.tree-item.file.active {
  background: var(--color-amber-muted);
  color: var(--color-fg-accent);
  font-weight: var(--weight-medium);
}

.tree-item.dir {
  color: var(--color-fg-secondary);
  font-weight: var(--weight-medium);
}

.indent {
  flex-shrink: 0;
  width: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.chevron {
  color: var(--color-fg-muted);
  transition: transform var(--duration-fast) var(--ease-out);
}

.chevron.expanded {
  transform: rotate(90deg);
}

.icon {
  flex-shrink: 0;
  color: var(--color-fg-muted);
}

.tree-item.dir .icon {
  color: var(--color-fg-secondary);
}

.label {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
