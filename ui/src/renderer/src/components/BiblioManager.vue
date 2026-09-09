<script setup lang="ts">
import { ref } from 'vue'
import { marked } from 'marked'
import BiblioTreeItem from './BiblioTreeItem.vue'
import type { BiblioTree } from '../../../preload'

const props = defineProps<{
  tree: BiblioTree | null
  projectPath: string
}>()

const selectedPath = ref<string | null>(null)
const html = ref('')
const loading = ref(false)

async function selectFile(path: string): Promise<void> {
  selectedPath.value = path
  loading.value = true
  const raw = await window.api.biblio.read(props.projectPath, path)
  html.value =
    raw !== null ? await marked.parse(raw) : '<p class="error">Could not read this file.</p>'
  loading.value = false
}

const treeWidth = ref(240)
let dragStartX = 0
let dragStartWidth = 0

function onGutterDown(event: PointerEvent): void {
  dragStartX = event.clientX
  dragStartWidth = treeWidth.value
  window.addEventListener('pointermove', onGutterMove)
  window.addEventListener('pointerup', onGutterUp)
}

function onGutterMove(event: PointerEvent): void {
  const delta = event.clientX - dragStartX
  treeWidth.value = Math.min(480, Math.max(180, dragStartWidth + delta))
}

function onGutterUp(): void {
  window.removeEventListener('pointermove', onGutterMove)
  window.removeEventListener('pointerup', onGutterUp)
}
</script>

<template>
  <div class="biblio-pane">
    <div class="biblio-tree" :style="{ width: `${treeWidth}px` }">
      <button
        v-if="tree?.toc"
        class="toc-item"
        :class="{ active: tree.toc.path === selectedPath }"
        @click="selectFile(tree.toc.path)"
      >
        <svg
          width="13"
          height="13"
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <line x1="2" y1="3" x2="12" y2="3" />
          <line x1="2" y1="7" x2="12" y2="7" />
          <line x1="2" y1="11" x2="8" y2="11" />
        </svg>
        <span>Table of Content</span>
      </button>

      <div class="tree-groups">
        <template v-if="tree?.children?.length">
          <BiblioTreeItem
            v-for="child in tree.children"
            :key="child.path"
            :node="child"
            :selected-path="selectedPath"
            :depth="0"
            @select="selectFile"
          />
        </template>
        <p v-else class="empty">Nothing in biblio/ yet.</p>
      </div>
    </div>

    <div class="biblio-gutter" @pointerdown="onGutterDown" />

    <div class="biblio-reader">
      <p v-if="!selectedPath" class="empty">Select a file to read it.</p>
      <p v-else-if="loading" class="empty">Loading...</p>
      <div v-else class="markdown-body" v-html="html" />
    </div>
  </div>
</template>

<style scoped>
.biblio-pane {
  flex: 1;
  min-height: 0;
  display: flex;
}

.biblio-tree {
  flex-shrink: 0;
  overflow-y: auto;
  padding: var(--space-3) var(--space-2) var(--space-4);
  background: var(--color-surface-1);
  border-radius: var(--radius-lg) 0 0 var(--radius-lg);
}

.toc-item {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: var(--space-2) var(--space-3);
  margin-bottom: var(--space-3);
  background: var(--color-surface-2);
  border: none;
  border-radius: var(--radius-md);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.toc-item:hover {
  background: var(--color-surface-3);
}

.toc-item.active {
  background: var(--color-amber-muted);
  color: var(--color-fg-accent);
}

.tree-groups {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.biblio-gutter {
  flex-shrink: 0;
  width: 4px;
  cursor: col-resize;
  background: var(--color-border-subtle);
  transition: background var(--duration-fast) var(--ease-out);
}

.biblio-gutter:hover,
.biblio-gutter:active {
  background: var(--color-amber);
}

.biblio-reader {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: var(--space-8);
  display: flex;
  justify-content: center;
}

.empty {
  color: var(--color-fg-muted);
  font-size: var(--text-sm);
  padding: var(--space-4);
}

.markdown-body {
  width: 100%;
  max-width: 720px;
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3) {
  font-family: var(--font-display);
  font-weight: var(--weight-bold);
  line-height: var(--leading-tight);
  margin-top: var(--space-8);
  margin-bottom: var(--space-3);
}

.markdown-body :deep(h1:first-child),
.markdown-body :deep(h2:first-child),
.markdown-body :deep(h3:first-child) {
  margin-top: 0;
}

.markdown-body :deep(h1) {
  font-size: var(--text-xl);
}
.markdown-body :deep(h2) {
  font-size: var(--text-lg);
}
.markdown-body :deep(h3) {
  font-size: var(--text-md);
}

.markdown-body :deep(p) {
  margin-bottom: var(--space-4);
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin-bottom: var(--space-4);
  padding-left: var(--space-6);
}

.markdown-body :deep(li) {
  margin-bottom: var(--space-1);
}

.markdown-body :deep(a) {
  color: var(--color-fg-accent);
}

.markdown-body :deep(strong) {
  font-weight: var(--weight-semibold);
}

.markdown-body :deep(code) {
  font-family: var(--font-mono);
  font-size: 0.9em;
  background: var(--color-surface-2);
  padding: 0.15em 0.4em;
  border-radius: var(--radius-sm);
}

.markdown-body :deep(pre) {
  background: var(--color-surface-2);
  border-radius: var(--radius-lg);
  padding: var(--space-4);
  overflow-x: auto;
  margin-bottom: var(--space-4);
}

.markdown-body :deep(pre code) {
  background: none;
  padding: 0;
}

.markdown-body :deep(blockquote) {
  border-left: 2px solid var(--color-border-default);
  padding-left: var(--space-4);
  color: var(--color-fg-secondary);
  margin-bottom: var(--space-4);
}

.markdown-body :deep(hr) {
  border: none;
  border-top: 1px solid var(--color-border-subtle);
  margin: var(--space-6) 0;
}
</style>
