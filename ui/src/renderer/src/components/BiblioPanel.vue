<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  AppButton,
  AppCheckbox,
  AppEmptyState,
  AppIconButton,
  AppInput,
  AppModal,
  AppNavItem,
  AppSwitcher
} from '@sankasten'
import BiblioTreeItem from './BiblioTreeItem.vue'
import MarkdownEditor from './MarkdownEditor.vue'
import TreeSearchBar from './treeSearch/TreeSearchBar.vue'
import { activePath, closeUnder, openFile, type ProjectRef } from '../workspace'
import { biblioExpandedOf } from '../explorerStore'
import { useBiblioSearch } from '../useBiblioSearch'
import type { BiblioScope } from '../biblioSearch'
import type { BiblioTree } from '../../../preload/types'

const props = defineProps<{
  tree: BiblioTree | null
  project: ProjectRef
}>()

const emit = defineEmits<{
  refresh: []
}>()

const selectedPath = computed((): string | null => activePath(props.project.id))

function selectFile(path: string): void {
  openFile(props.project, path)
}

function toggleDir(path: string): void {
  const expanded = biblioExpandedOf(props.project.id)
  if (expanded[path]) delete expanded[path]
  else expanded[path] = true
}

function expandDir(path: string): void {
  biblioExpandedOf(props.project.id)[path] = true
}

const panel = ref<HTMLElement | null>(null)

const search = useBiblioSearch({
  panel,
  nodes: () => props.tree?.children ?? [],
  expanded: () => biblioExpandedOf(props.project.id),
  openFile: selectFile,
  toggleOwn: toggleDir
})

const scopeItems: { id: BiblioScope; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'books', label: 'Books' },
  { id: 'scratchpad', label: 'Scratchpad' },
  { id: 'handover', label: 'Handover' },
  { id: 'laws', label: 'Laws' }
]

function pickScope(id: string): void {
  search.scope.value = id as BiblioScope
}

// The tree refetches when biblio/ changes on disk, and an open search follows it.
watch(
  () => props.tree,
  () => search.refresh()
)

async function archiveScratchpad(taskPath: string): Promise<void> {
  const ok = await window.api.biblio.archiveScratchpad(props.project.path, taskPath)
  if (!ok) return
  closeUnder(props.project, taskPath)
  emit('refresh')
}

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
      props.project.path,
      notePath,
      bytes,
      file.type
    )
    if ('path' in result) resolved = resolved.replaceAll(url, result.path)
  }
  if (resolved !== content) await window.api.biblio.write(props.project.path, notePath, resolved)
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

function cancelCreate(): void {
  clearPendingImages()
  showCreateForm.value = false
}

async function submitCreate(): Promise<void> {
  if (!newTitle.value.trim() || creating.value) return
  creating.value = true
  const path = createTargetFolder.value
    ? await window.api.biblio.createScratchpadFile(
        props.project.path,
        createTargetFolder.value,
        newTitle.value,
        newContent.value
      )
    : await window.api.biblio.createScratchpad(props.project.path, newTitle.value, newContent.value)
  if (path) await storePendingImages(path, newContent.value)
  creating.value = false
  if (!path) return
  clearPendingImages()
  showCreateForm.value = false
  emit('refresh')
  selectFile(path)
}
</script>

<template>
  <div ref="panel" class="biblio-view" tabindex="0" @keydown="search.onPanelKeydown">
    <div class="biblio-head">
      <span class="head-label">biblio</span>
      <AppIconButton
        size="sm"
        tone="accent"
        icon="search"
        label="Search"
        title="Search (or start typing)"
        :pressed="search.open.value"
        @click="search.show()"
      />
    </div>
    <div class="tree-scroll">
      <AppNavItem
        v-if="tree?.toc"
        class="toc-item"
        size="sm"
        label="Table of Content"
        :active="tree.toc.path === selectedPath"
        @click="selectFile(tree.toc.path)"
      >
        <template #icon>
          <svg
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
        </template>
      </AppNavItem>

      <TreeSearchBar
        v-if="search.open.value"
        :search="search"
        :status="search.status.value"
        placeholder="Search biblio"
      >
        <div class="search-options">
          <AppSwitcher
            size="sm"
            :items="scopeItems"
            :active-id="search.scope.value"
            placeholder="Scope"
            empty-text="No scopes."
            @select="pickScope"
          />
          <AppCheckbox
            size="sm"
            :checked="search.includeArchive.value"
            @click="search.includeArchive.value = !search.includeArchive.value"
          >
            Include archive
          </AppCheckbox>
        </div>
      </TreeSearchBar>

      <div class="tree-groups">
        <template v-if="tree?.children?.length">
          <BiblioTreeItem
            v-for="child in search.result.value?.tree ?? tree.children"
            :key="child.path"
            :node="child"
            :selected-path="selectedPath"
            :depth="0"
            :expanded-paths="search.expandedPaths.value"
            :matches="search.result.value?.matches"
            :cursor-path="search.cursorPath.value"
            :no-match="search.result.value?.noMatch"
            @select="selectFile"
            @toggle="search.toggle"
            @expand="expandDir"
            @add-scratchpad="openCreateForm"
            @add-scratchpad-file="openCreateFileForm"
            @archive-scratchpad="archiveScratchpad"
          />
        </template>
        <AppEmptyState v-else class="tree-empty" size="sm" title="Nothing in biblio/ yet." />
      </div>
    </div>

    <AppModal
      v-if="showCreateForm"
      scope="app"
      size="md"
      :title="createTargetFolder ? 'New file' : 'New scratchpad'"
      @close="cancelCreate"
    >
      <AppInput
        v-model="newTitle"
        type="text"
        placeholder="Title"
        autofocus
        @keydown.enter="submitCreate"
      />
      <MarkdownEditor
        v-model="newContent"
        @image-pending="(url, file) => pendingImages.set(url, file)"
      />
      <template #footer>
        <AppButton @click="cancelCreate">Cancel</AppButton>
        <AppButton variant="primary" :disabled="!newTitle.trim() || creating" @click="submitCreate">
          {{ creating ? 'Creating...' : 'Create' }}
        </AppButton>
      </template>
    </AppModal>
  </div>
</template>

<style scoped>
.biblio-view {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  outline: none;
}

.biblio-head {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: var(--space-2) var(--space-3) var(--space-1) var(--space-3);
}

.head-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
}

.tree-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--space-3) var(--space-2) var(--space-4);
}

.toc-item {
  margin-bottom: var(--space-3);
}

.search-options {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: 0 var(--space-1);
}

.tree-groups {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tree-empty {
  margin: var(--space-2);
}
</style>
