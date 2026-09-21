<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { marked } from 'marked'
import BiblioTreeItem from './BiblioTreeItem.vue'
import MarkdownEditor from './MarkdownEditor.vue'
import type { BiblioTree } from '../../../preload'

const props = defineProps<{
  tree: BiblioTree | null
  projectPath: string
}>()

const emit = defineEmits<{
  refresh: []
}>()

const selectedPath = ref<string | null>(null)
const html = ref('')
const editableContent = ref('')
const loading = ref(false)

// Scratchpad is the one tier bibliothek calls user-editable -- every other
// category stays read-only rendered markdown.
const isEditable = computed((): boolean => selectedPath.value?.startsWith('scratchpad/') ?? false)

let suppressAutosave = false

async function selectFile(path: string): Promise<void> {
  selectedPath.value = path
  loading.value = true
  const raw = await window.api.biblio.read(props.projectPath, path)
  if (path.startsWith('scratchpad/')) {
    suppressAutosave = true
    editableContent.value = raw ?? ''
  } else {
    html.value =
      raw !== null ? await marked.parse(raw) : '<p class="error">Could not read this file.</p>'
  }
  loading.value = false
}

let saveTimer: ReturnType<typeof setTimeout> | undefined

watch(editableContent, (value) => {
  if (suppressAutosave) {
    suppressAutosave = false
    return
  }
  if (!selectedPath.value) return
  const path = selectedPath.value
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    window.api.biblio.write(props.projectPath, path, value)
  }, 600)
})

const showCreateForm = ref(false)
const newTitle = ref('')
const newContent = ref('')
const creating = ref(false)
const pendingImages = new Map<string, File>()

function clearPendingImages(): void {
  for (const url of pendingImages.keys()) URL.revokeObjectURL(url)
  pendingImages.clear()
}

async function storePendingImages(notePath: string, content: string): Promise<void> {
  let resolved = content
  for (const [url, file] of pendingImages) {
    if (!resolved.includes(url)) continue
    const bytes = new Uint8Array(await file.arrayBuffer())
    const result = await window.api.biblio.saveScratchpadImage(
      props.projectPath,
      notePath,
      bytes,
      file.type
    )
    if ('path' in result) resolved = resolved.replaceAll(url, result.path)
  }
  if (resolved !== content) await window.api.biblio.write(props.projectPath, notePath, resolved)
}
// Non-null when the create form targets an existing folder instead of a new task directory.
const createTargetFolder = ref<string | null>(null)

function openCreateForm(): void {
  clearPendingImages()
  newTitle.value = ''
  newContent.value = ''
  createTargetFolder.value = null
  showCreateForm.value = true
}

function openCreateFileForm(folderPath: string): void {
  clearPendingImages()
  newTitle.value = ''
  newContent.value = ''
  createTargetFolder.value = folderPath
  showCreateForm.value = true
}

async function archiveScratchpad(taskPath: string): Promise<void> {
  const ok = await window.api.biblio.archiveScratchpad(props.projectPath, taskPath)
  if (!ok) return
  if (selectedPath.value === taskPath || selectedPath.value?.startsWith(`${taskPath}/`)) {
    selectedPath.value = null
  }
  emit('refresh')
}

function cancelCreate(): void {
  clearPendingImages()
  showCreateForm.value = false
}

async function submitCreate(): Promise<void> {
  if (!newTitle.value.trim() || creating.value) return
  creating.value = true
  const path = createTargetFolder.value
    ? await window.api.biblio.createScratchpadFile(
        props.projectPath,
        createTargetFolder.value,
        newTitle.value,
        newContent.value
      )
    : await window.api.biblio.createScratchpad(props.projectPath, newTitle.value, newContent.value)
  if (path) await storePendingImages(path, newContent.value)
  creating.value = false
  if (!path) return
  clearPendingImages()
  showCreateForm.value = false
  emit('refresh')
  await selectFile(path)
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
            @add-scratchpad="openCreateForm"
            @add-scratchpad-file="openCreateFileForm"
            @archive-scratchpad="archiveScratchpad"
          />
        </template>
        <p v-else class="empty">Nothing in biblio/ yet.</p>
      </div>
    </div>

    <div class="biblio-gutter" @pointerdown="onGutterDown" />

    <div class="biblio-reader">
      <p v-if="!selectedPath" class="empty">Select a file to read it.</p>
      <p v-else-if="loading" class="empty">Loading...</p>
      <MarkdownEditor
        v-else-if="isEditable"
        v-model="editableContent"
        :project-path="projectPath"
        :note-path="selectedPath ?? undefined"
      />
      <div v-else class="markdown-body" v-html="html" />
    </div>

    <div v-if="showCreateForm" class="create-overlay">
      <div class="create-card">
        <h3 class="create-title">{{ createTargetFolder ? 'New file' : 'New scratchpad' }}</h3>
        <input
          v-model="newTitle"
          class="title-input"
          type="text"
          placeholder="Title"
          autofocus
          @keydown.enter="submitCreate"
        />
        <MarkdownEditor
          v-model="newContent"
          @image-pending="(url, file) => pendingImages.set(url, file)"
        />
        <div class="create-actions">
          <button class="ghost-button" @click="cancelCreate">Cancel</button>
          <button
            class="primary-button"
            :disabled="!newTitle.trim() || creating"
            @click="submitCreate"
          >
            {{ creating ? 'Creating...' : 'Create' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.biblio-pane {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
}

.create-overlay {
  position: absolute;
  inset: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-8);
  background: var(--color-bg-overlay);
}

.create-card {
  width: 100%;
  max-width: 640px;
  max-height: 100%;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-6);
  background: var(--color-surface-1);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-xl);
}

.create-title {
  font-family: var(--font-display);
  font-size: var(--text-md);
  font-weight: var(--weight-semibold);
  color: var(--color-fg-primary);
}

.title-input {
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-2);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
}

.title-input:focus {
  outline: none;
  border-color: var(--color-amber);
}

.create-actions {
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

.primary-button:hover:not(:disabled) {
  background: var(--color-amber-dim);
}

.primary-button:disabled {
  opacity: 0.5;
  cursor: default;
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
