<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The server tab's file browser. It overlays the right side of the terminal instead of
// splitting the tab, so the terminal never resizes and a full-screen program in it never redraws.
// Its folder, history, clipboard and transfers live in filesStore, keyed by the tab, so a remount
// keeps them.
import { computed, onMounted, ref } from 'vue'
import { AppButton, AppContextMenu, AppEmptyState, AppIconButton, type MenuItem } from '@sankasten'
import { sshIcon } from '../../../icons'
import FilesNavbar from './FilesNavbar.vue'
import FilesStatus from './FilesStatus.vue'
import FilesView from './FilesView.vue'
import NameModal from './NameModal.vue'
import { FILES_PATHS } from './drawerIcons'
import {
  cancelTransfer,
  createEntry,
  downloadEntry,
  filesOf,
  load,
  navigate,
  paste,
  removeEntry,
  renameEntry,
  setClipboard,
  uploadFiles,
  uploadPicked
} from './filesStore'
import { joinRemote } from './remotePath'
import { openServerFile } from '../editor/remoteFiles'
import type { SSHFileEntry } from '../../../../../preload/types'

const props = defineProps<{ tabId: string; alias: string }>()

const emit = defineEmits<{ close: [] }>()

interface NameAsk {
  title: string
  initial: string
  apply: (name: string) => Promise<string | null>
}

const state = computed(() => filesOf(props.tabId, props.alias))
const showList = computed(() => state.value.loaded && !state.value.blocked)
const selected = ref<string | null>(null)
const menu = ref<{ x: number; y: number; entry: SSHFileEntry | null } | null>(null)
const ask = ref<NameAsk | null>(null)
const dropping = ref(false)

onMounted(() => {
  if (!state.value.loaded && !state.value.loading) void navigate(props.tabId, null)
})

function retry(): void {
  if (state.value.history.length) void load(props.tabId)
  else void navigate(props.tabId, null)
}

// A folder is entered; a file opens in the server tab's editor, beside the layer.
function open(entry: SSHFileEntry): void {
  const path = joinRemote(state.value.path ?? '/', entry.name)
  if (!entry.dir) {
    openServerFile(props.tabId, props.alias, path)
    return
  }
  selected.value = null
  void navigate(props.tabId, path)
}

const menuItems = computed((): MenuItem[] => {
  const entry = menu.value?.entry
  if (entry) {
    const items: MenuItem[] = entry.dir
      ? []
      : [{ key: 'open', label: 'Open', icon: FILES_PATHS.open }]
    return [
      ...items,
      { key: 'move', label: 'Move', icon: FILES_PATHS.move },
      { key: 'copy', label: 'Copy', icon: FILES_PATHS.copy },
      { key: 'download', label: 'Download', icon: FILES_PATHS.download },
      { key: 'rename', label: 'Rename', icon: FILES_PATHS.rename },
      { key: 'delete', label: 'Delete', icon: FILES_PATHS.trash, confirm: true }
    ]
  }
  const items: MenuItem[] = [
    { key: 'newFolder', label: 'New folder', icon: FILES_PATHS.newFolder },
    { key: 'newFile', label: 'New file', icon: FILES_PATHS.newFile },
    { key: 'upload', label: 'Upload', icon: FILES_PATHS.upload }
  ]
  const clip = state.value.clipboard
  if (clip) items.push({ key: 'paste', label: `Paste ${clip.name}`, icon: FILES_PATHS.paste })
  return items
})

function onMenu(event: MouseEvent, entry: SSHFileEntry | null): void {
  if (!showList.value) return
  menu.value = { x: event.clientX, y: event.clientY, entry }
}

function onSelect(key: string): void {
  const entry = menu.value?.entry ?? null
  menu.value = null
  const tabId = props.tabId
  if (key === 'newFolder') {
    ask.value = { title: 'New folder', initial: '', apply: (n) => createEntry(tabId, 'folder', n) }
  } else if (key === 'newFile') {
    ask.value = { title: 'New file', initial: '', apply: (n) => createEntry(tabId, 'file', n) }
  } else if (key === 'upload') void uploadPicked(tabId)
  else if (key === 'paste') void paste(tabId)
  else if (!entry) return
  else if (key === 'open') open(entry)
  else if (key === 'move' || key === 'copy') setClipboard(tabId, entry.name, key)
  else if (key === 'download') void downloadEntry(tabId, entry)
  else if (key === 'rename') {
    ask.value = {
      title: `Rename ${entry.name}`,
      initial: entry.name,
      apply: (n) => renameEntry(tabId, entry.name, n)
    }
  } else if (key === 'delete') void removeEntry(tabId, entry.name)
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

const folderName = computed(() => {
  const path = state.value.path ?? ''
  return path === '/' ? '/' : (path.split('/').pop() ?? '')
})
</script>

<template>
  <aside
    class="files-layer"
    aria-label="Files"
    @dragover="onDragOver"
    @dragleave="onDragLeave"
    @drop="onDrop"
  >
    <FilesNavbar :tab-id="tabId" :state="state" @close="emit('close')" />

    <div v-if="state.sudo" class="banner root" role="status">Acting as root</div>
    <div v-if="state.notice" class="banner warn" role="alert">
      <span class="banner-text">{{ state.notice }}</span>
      <AppIconButton size="xs" icon="close" label="Dismiss" @click="state.notice = ''" />
    </div>

    <div class="layer-body">
      <AppEmptyState
        v-if="state.blocked === 'notLoggedIn'"
        layout="fill"
        :icon="sshIcon"
        title="Log in in the terminal first."
        detail="Files uses the terminal's connection."
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
      <div
        v-else-if="!state.entries.length"
        class="empty"
        @contextmenu.prevent="onMenu($event, null)"
      >
        <AppEmptyState
          size="sm"
          title="The folder is empty."
          detail="Drop files here or right-click to add some."
        />
      </div>
      <FilesView
        v-else
        :entries="state.entries"
        :view="state.view"
        :selected="selected"
        @select="(name) => (selected = name)"
        @open="open"
        @menu="onMenu"
      />
    </div>

    <FilesStatus
      :slow="state.slow"
      :transfers="state.transfers"
      @cancel="(id) => cancelTransfer(tabId, id)"
    />

    <div v-if="dropping" class="drop-veil">
      <span>Drop to upload into {{ folderName }}</span>
    </div>

    <AppContextMenu
      v-if="menu"
      :x="menu.x"
      :y="menu.y"
      :items="menuItems"
      @select="onSelect"
      @close="menu = null"
    />

    <NameModal
      v-if="ask"
      :title="ask.title"
      :initial="ask.initial"
      :apply="ask.apply"
      @close="ask = null"
    />
  </aside>
</template>

<style scoped>
.files-layer {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 2;
  display: flex;
  flex-direction: column;
  width: max(320px, 45%);
  max-width: 100%;
  container: files-layer / inline-size;
  border-left: 1px solid var(--color-border-default);
  background: var(--color-surface-1);
  box-shadow: var(--shadow-lg);
  font-family: var(--font-body);
}

.banner {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 28px;
  padding: 0 var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
  font-size: var(--text-xs);
}

.banner.root {
  border-bottom-color: var(--color-error);
  background: var(--color-error-muted);
  color: var(--color-error);
  font-weight: var(--weight-medium);
}

.banner.warn {
  color: var(--color-error);
}

.banner-text {
  flex: 1;
  min-width: 0;
  padding: var(--space-1) 0;
}

.layer-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}

.empty {
  flex: 1;
  padding: var(--space-2);
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
