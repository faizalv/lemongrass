<script setup lang="ts">
import { ref } from 'vue'
import TerminalGroup from './TerminalGroup.vue'
import type { PaneLayoutNode } from '../../../preload'

// Recursive renderer for the split-pane tree: a leaf becomes a
// TerminalGroup, a split becomes its children laid out side by side
// with a hand-dragged gutter between each pair.

const props = defineProps<{
  node: PaneLayoutNode
  focusedPaneId: string | null
  titles: Record<string, string>
}>()

const emit = defineEmits<{
  focus: [paneId: string]
  'select-tab': [paneId: string, tabId: string]
  'close-tab': [paneId: string, tabId: string]
  'add-tab': [paneId: string]
  split: [paneId: string, direction: 'row' | 'column']
  'close-pane': [paneId: string]
  exit: [paneId: string, tabId: string]
  resize: [splitId: string, sizes: number[]]
  'title-change': [paneId: string, tabId: string, title: string]
}>()

const containerEl = ref<HTMLDivElement>()
let dragIndex = -1
let dragStartPos = 0
let dragStartSizes: number[] = []

function onGutterDown(index: number, event: PointerEvent): void {
  if (props.node.type !== 'split') return
  dragIndex = index
  dragStartPos = props.node.direction === 'row' ? event.clientX : event.clientY
  dragStartSizes = [...props.node.sizes]
  window.addEventListener('pointermove', onGutterMove)
  window.addEventListener('pointerup', onGutterUp)
}

function onGutterMove(event: PointerEvent): void {
  const node = props.node
  if (node.type !== 'split' || !containerEl.value) return
  const rect = containerEl.value.getBoundingClientRect()
  const total = node.direction === 'row' ? rect.width : rect.height
  const pos = node.direction === 'row' ? event.clientX : event.clientY
  const delta = (pos - dragStartPos) / total

  const a = dragIndex
  const b = dragIndex + 1
  const pairTotal = dragStartSizes[a] + dragStartSizes[b]
  const min = Math.min(0.1, pairTotal / 2)
  const nextA = Math.max(min, Math.min(pairTotal - min, dragStartSizes[a] + delta))

  const sizes = [...dragStartSizes]
  sizes[a] = nextA
  sizes[b] = pairTotal - nextA
  emit('resize', node.id, sizes)
}

function onGutterUp(): void {
  dragIndex = -1
  window.removeEventListener('pointermove', onGutterMove)
  window.removeEventListener('pointerup', onGutterUp)
}
</script>

<template>
  <TerminalGroup
    v-if="node.type === 'leaf'"
    :tabs="node.tabs"
    :active-tab-id="node.activeTabId"
    :focused="node.id === focusedPaneId"
    :titles="titles"
    @focus="emit('focus', node.id)"
    @select-tab="(tabId) => emit('select-tab', node.id, tabId)"
    @close-tab="(tabId) => emit('close-tab', node.id, tabId)"
    @add-tab="emit('add-tab', node.id)"
    @split="(direction) => emit('split', node.id, direction)"
    @close-pane="emit('close-pane', node.id)"
    @exit="(tabId) => emit('exit', node.id, tabId)"
    @title-change="(tabId, title) => emit('title-change', node.id, tabId, title)"
  />

  <div v-else ref="containerEl" class="split" :class="node.direction">
    <template v-for="(child, index) in node.children" :key="child.id">
      <div class="split-child" :style="{ flex: `0 1 ${node.sizes[index] * 100}%` }">
        <PaneLayout
          :node="child"
          :focused-pane-id="focusedPaneId"
          :titles="titles"
          @focus="(id) => emit('focus', id)"
          @select-tab="(id, tabId) => emit('select-tab', id, tabId)"
          @close-tab="(id, tabId) => emit('close-tab', id, tabId)"
          @add-tab="(id) => emit('add-tab', id)"
          @split="(id, direction) => emit('split', id, direction)"
          @close-pane="(id) => emit('close-pane', id)"
          @exit="(id, tabId) => emit('exit', id, tabId)"
          @resize="(splitId, sizes) => emit('resize', splitId, sizes)"
          @title-change="(id, tabId, title) => emit('title-change', id, tabId, title)"
        />
      </div>
      <div
        v-if="index < node.children.length - 1"
        class="gutter"
        :class="node.direction"
        @pointerdown="onGutterDown(index, $event)"
      />
    </template>
  </div>
</template>

<style scoped>
.split {
  height: 100%;
  width: 100%;
  min-height: 0;
  min-width: 0;
  display: flex;
}

.split.row {
  flex-direction: row;
}

.split.column {
  flex-direction: column;
}

.split-child {
  min-height: 0;
  min-width: 0;
  overflow: hidden;
  display: flex;
}

.gutter {
  flex-shrink: 0;
  background: var(--color-border-subtle);
  transition: background var(--duration-fast) var(--ease-out);
}

.gutter:hover,
.gutter:active {
  background: var(--color-amber);
}

.gutter.row {
  width: 4px;
  cursor: col-resize;
}

.gutter.column {
  height: 4px;
  cursor: row-resize;
}
</style>
