<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The server tab's file browser. It overlays the right side of the terminal instead of
// splitting the tab, so the terminal never resizes and a full-screen program in it never redraws.
// Its folder and transfers live in filesStore, keyed by the tab, so a remount keeps them.
import { computed, nextTick, onMounted, ref } from 'vue'
import {
  AppButton,
  AppEmptyState,
  AppIconButton,
  AppInput,
  AppProgressLine,
  type AppInputHandle
} from '@sankasten'
import TabIcon from '../../TabIcon.vue'
import { folderIconOf } from '../../../fileIcons'
import { sshIcon } from '../../../icons'
import FileRow from './FileRow.vue'
import TransferStrip from './TransferStrip.vue'
import { DRAWER_ICONS } from './drawerIcons'
import {
  cancelTransfer,
  dismissTransfer,
  downloadEntry,
  filesOf,
  load,
  makeFolder,
  removeEntry,
  renameEntry,
  setSudo,
  transferRunning,
  uploadFiles
} from './filesStore'
import { crumbsOf, joinRemote, nameProblem, parentOf } from './remotePath'

const props = defineProps<{ tabId: string; alias: string }>()

const state = computed(() => filesOf(props.tabId, props.alias))
const crumbs = computed(() => (state.value.path ? crumbsOf(state.value.path) : []))
const atRoot = computed(() => state.value.path === '/')
const showList = computed(() => state.value.loaded && !state.value.blocked)

const crumbsEl = ref<HTMLElement | null>(null)
const creating = ref(false)
const newName = ref('')
const newInput = ref<AppInputHandle | null>(null)
const newProblem = computed(() => (newName.value ? nameProblem(newName.value.trim()) : null))
const dropping = ref(false)

onMounted(() => {
  if (!state.value.loaded && !state.value.loading) void open(null)
})

async function open(path: string | null): Promise<void> {
  creating.value = false
  await load(props.tabId, path)
  await nextTick()
  // A deep path keeps its last folders in view.
  if (crumbsEl.value) crumbsEl.value.scrollLeft = crumbsEl.value.scrollWidth
}

function retry(): void {
  void open(state.value.path)
}

async function startCreate(): Promise<void> {
  newName.value = ''
  creating.value = true
  await nextTick()
  newInput.value?.focus()
}

async function commitCreate(): Promise<void> {
  const name = newName.value.trim()
  if (!name || nameProblem(name)) return
  if (await makeFolder(props.tabId, name)) creating.value = false
}

function onCreateKey(event: KeyboardEvent): void {
  if (event.key === 'Enter') void commitCreate()
  else if (event.key === 'Escape') creating.value = false
}

function isFileDrag(event: DragEvent): boolean {
  return event.dataTransfer?.types.includes('Files') ?? false
}

function onDragOver(event: DragEvent): void {
  if (!isFileDrag(event) || !showList.value) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
  dropping.value = true
}

function onDragLeave(event: DragEvent): void {
  const root = event.currentTarget as HTMLElement
  if (!root.contains(event.relatedTarget as Node | null)) dropping.value = false
}

function onDrop(event: DragEvent): void {
  dropping.value = false
  if (!isFileDrag(event) || !showList.value) return
  event.preventDefault()
  const paths = [...(event.dataTransfer?.files ?? [])]
    .map((file) => window.api.ssh.session.pathForFile(file))
    .filter((path) => !!path)
  if (paths.length) void uploadFiles(props.tabId, paths)
}

const busyTransfers = computed(() => transferRunning(state.value))

const folderName = computed(() => crumbs.value.at(-1)?.name ?? '')
const folderIcon = folderIconOf('', false)
</script>

<template>
  <aside
    class="files-drawer"
    aria-label="Files"
    @dragover="onDragOver"
    @dragleave="onDragLeave"
    @drop="onDrop"
  >
    <header class="drawer-head" :class="{ root: state.sudo }">
      <div class="head-row">
        <nav ref="crumbsEl" class="crumbs" aria-label="Folder path">
          <template v-for="(crumb, index) in crumbs" :key="crumb.path">
            <span v-if="index > 1" class="sep">/</span>
            <button
              type="button"
              class="crumb"
              :class="{ current: index === crumbs.length - 1 }"
              :title="crumb.path"
              @click="open(crumb.path)"
            >
              {{ crumb.name }}
            </button>
          </template>
        </nav>
        <div class="tools">
          <AppIconButton
            size="sm"
            label="Up"
            :disabled="!showList || atRoot"
            @click="state.path && open(parentOf(state.path))"
          >
            <TabIcon :shapes="DRAWER_ICONS.up" />
          </AppIconButton>
          <AppIconButton size="sm" label="Refresh" :busy="state.loading" @click="retry">
            <TabIcon :shapes="DRAWER_ICONS.refresh" />
          </AppIconButton>
          <AppIconButton
            size="sm"
            tone="accent"
            label="New folder"
            :disabled="!showList"
            @click="startCreate"
          >
            <TabIcon :shapes="DRAWER_ICONS.newFolder" />
          </AppIconButton>
          <AppIconButton
            size="sm"
            tone="danger"
            :label="state.sudo ? 'Act as the login user' : 'Act as root'"
            :title="busyTransfers ? 'Wait for the transfers to finish.' : undefined"
            :pressed="state.sudo"
            :disabled="busyTransfers || state.loading"
            @click="setSudo(tabId, !state.sudo)"
          >
            <span class="root-glyph">#</span>
          </AppIconButton>
        </div>
      </div>
      <div v-if="state.sudo" class="root-line">Acting as root</div>
    </header>

    <AppProgressLine v-if="state.loading && state.loaded" class="loading" label="Loading folder" />

    <div v-if="state.notice && showList" class="notice" role="alert">
      <span class="notice-text">{{ state.notice }}</span>
      <AppIconButton size="xs" icon="close" label="Dismiss" @click="state.notice = ''" />
    </div>

    <div class="drawer-body">
      <AppEmptyState
        v-if="state.blocked === 'notLoggedIn'"
        layout="fill"
        :icon="sshIcon"
        title="Log in in the terminal first."
        detail="The file browser uses the terminal's connection."
      >
        <template #action>
          <AppButton :disabled="state.loading" @click="retry">Retry</AppButton>
        </template>
      </AppEmptyState>
      <AppEmptyState
        v-else-if="state.blocked === 'error'"
        layout="fill"
        :icon="sshIcon"
        title="The folder cannot be listed."
        :detail="state.blockedText"
      >
        <template #action>
          <AppButton :disabled="state.loading" @click="retry">Retry</AppButton>
        </template>
      </AppEmptyState>
      <AppEmptyState v-else-if="!state.loaded" layout="fill" loading title="Opening files" />
      <div v-else class="list" role="list">
        <div v-if="creating" class="create-row">
          <TabIcon :shapes="folderIcon.shapes" :style="{ color: folderIcon.color }" />
          <AppInput
            ref="newInput"
            v-model="newName"
            class="create-input"
            size="sm"
            placeholder="Folder name"
            aria-label="New folder name"
            :aria-invalid="!!newProblem"
            :title="newProblem ?? undefined"
            @keydown="onCreateKey"
            @blur="creating = false"
          />
        </div>
        <FileRow
          v-for="entry in state.entries"
          :key="joinRemote(state.path ?? '/', entry.name)"
          role="listitem"
          :entry="entry"
          @enter="open(joinRemote(state.path ?? '/', entry.name))"
          @download="downloadEntry(tabId, entry.name)"
          @rename="(to) => renameEntry(tabId, entry.name, to)"
          @remove="removeEntry(tabId, entry.name)"
        />
        <AppEmptyState
          v-if="!state.entries.length && !creating"
          size="sm"
          title="The folder is empty."
          detail="Drop files here to upload them."
        />
      </div>
    </div>

    <TransferStrip
      :transfers="state.transfers"
      @cancel="(id) => cancelTransfer(tabId, id)"
      @dismiss="(id) => dismissTransfer(tabId, id)"
    />

    <div v-if="dropping" class="drop-veil">
      <span>Drop to upload into {{ folderName }}</span>
    </div>
  </aside>
</template>

<style scoped>
.files-drawer {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 2;
  display: flex;
  flex-direction: column;
  width: max(320px, 45%);
  max-width: 100%;
  container: files-drawer / inline-size;
  border-left: 1px solid var(--color-border-default);
  background: var(--color-surface-1);
  box-shadow: var(--shadow-lg);
  font-family: var(--font-body);
}

.drawer-head {
  border-bottom: 1px solid var(--color-border-subtle);
}

.drawer-head.root {
  border-bottom-color: var(--color-error);
  background: var(--color-error-muted);
}

/* The Files toggle sits over the row's right end, so the row leaves room for it. */
.head-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 38px;
  padding: 0 96px 0 var(--space-3);
}

.root-line {
  padding: 0 var(--space-3) var(--space-2);
  color: var(--color-error);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
}

.root-glyph {
  font-family: var(--font-body);
  font-size: var(--text-sm);
  font-weight: var(--weight-bold);
  line-height: 1;
}

.crumbs {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  overflow-x: auto;
  scrollbar-width: none;
  white-space: nowrap;
}

.crumb {
  flex-shrink: 0;
  padding: 2px 4px;
  border: none;
  border-radius: var(--radius-sm);
  background: none;
  color: var(--color-fg-muted);
  font: inherit;
  font-size: var(--text-xs);
  cursor: pointer;
}

.crumb:hover {
  color: var(--color-fg-primary);
  background: var(--color-surface-2);
}

.crumb.current {
  color: var(--color-fg-primary);
  font-weight: var(--weight-medium);
}

.crumb:focus-visible {
  outline: 1px solid var(--color-amber);
}

.sep {
  color: var(--color-fg-disabled);
  font-size: var(--text-xs);
}

.tools {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.notice {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-1) var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
  color: var(--color-error);
  font-size: var(--text-xs);
}

.notice-text {
  flex: 1;
  min-width: 0;
}

.drawer-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}

.list {
  display: flex;
  flex-direction: column;
  gap: 1px;
  padding: var(--space-2);
}

.create-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: 0 var(--space-3);
}

.create-input {
  flex: 1;
  min-width: 0;
}

.drop-veil {
  position: absolute;
  inset: var(--space-2);
  z-index: 3;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed var(--color-amber);
  border-radius: var(--radius-lg);
  background: var(--color-surface-1);
  color: var(--color-fg-primary);
  font-size: var(--text-sm);
  pointer-events: none;
}
</style>
