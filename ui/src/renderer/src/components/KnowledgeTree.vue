<script setup lang="ts">
import { ref } from 'vue'
import type { TreeNode } from '../../../preload'

const props = defineProps<{
  nodes: TreeNode[]
  activeFilePath: string | null
}>()

const emit = defineEmits<{
  selectFile: [path: string]
}>()

const collapsed = ref<Set<string>>(new Set())

function toggle(path: string): void {
  if (collapsed.value.has(path)) collapsed.value.delete(path)
  else collapsed.value.add(path)
}

function onClick(node: TreeNode): void {
  if (node.type === 'dir') toggle(node.path)
  else emit('selectFile', node.path)
}
</script>

<template>
  <ul class="tree">
    <li v-for="node in props.nodes" :key="node.path">
      <button
        class="node"
        :class="{ dir: node.type === 'dir', active: node.path === props.activeFilePath }"
        @click="onClick(node)"
      >
        <span class="icon">{{
          node.type === 'dir' ? (collapsed.has(node.path) ? '▸' : '▾') : '·'
        }}</span>
        {{ node.name }}
      </button>
      <KnowledgeTree
        v-if="node.type === 'dir' && !collapsed.has(node.path) && node.children"
        :nodes="node.children"
        :active-file-path="props.activeFilePath"
        class="nested"
        @select-file="(p) => emit('selectFile', p)"
      />
    </li>
  </ul>
</template>

<style scoped>
.tree {
  list-style: none;
  margin: 0;
  padding: 0;
}

.tree.nested {
  padding-left: var(--space-4);
}

.node {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  width: 100%;
  text-align: left;
  padding: var(--space-1) var(--space-2);
  background: transparent;
  border: none;
  border-radius: var(--radius-sm);
  color: var(--color-fg-secondary);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  cursor: pointer;
}

.node:hover {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.node.active {
  background: var(--color-surface-3);
  color: var(--color-fg-accent);
}

.node.dir {
  color: var(--color-fg-primary);
}

.icon {
  color: var(--color-fg-muted);
  width: 10px;
  flex-shrink: 0;
}
</style>
