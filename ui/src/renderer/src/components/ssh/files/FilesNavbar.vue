<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The layer's navbar: Back and Forward through its own history, the path as an editable field,
// then Paste, Reload, the view and the root toggle, and Close alone at the right edge. Root mode
// colors only the banner below, never the navbar.
import { computed, ref, watch } from 'vue'
import { AppIconButton, AppInput, AppSwitcher } from '@sankasten'
import TabIcon from '../../TabIcon.vue'
import { FILES_PATHS, shapesOf } from './drawerIcons'
import {
  goHistory,
  load,
  navigate,
  paste,
  setSudo,
  setView,
  transferRunning,
  type FilesState,
  type FilesViewKind
} from './filesStore'

const props = defineProps<{ tabId: string; state: FilesState }>()

const emit = defineEmits<{ close: [] }>()

const VIEWS: { id: FilesViewKind; label: string }[] = [
  { id: 'detailed', label: 'Detailed' },
  { id: 'tile', label: 'Tile' },
  { id: 'basic', label: 'Basic' }
]

const draft = ref(props.state.path ?? '')
const editing = ref(false)
const ready = computed(() => props.state.loaded && !props.state.blocked)
const busyTransfers = computed(() => transferRunning(props.state))

// The field follows the shown folder unless the human is typing in it.
watch(
  () => props.state.path,
  (path) => {
    if (!editing.value) draft.value = path ?? ''
  }
)

async function go(): Promise<void> {
  const target = draft.value.trim()
  if (!target) return
  // A failure reports in the banner; the field returns to the shown folder either way.
  await navigate(props.tabId, target)
  draft.value = props.state.path ?? ''
}

function onKey(event: KeyboardEvent): void {
  if (event.key === 'Enter') void go()
  else if (event.key === 'Escape') {
    draft.value = props.state.path ?? ''
    ;(event.target as HTMLInputElement).blur()
  }
}

function onBlur(): void {
  editing.value = false
  draft.value = props.state.path ?? ''
}
</script>

<template>
  <header class="files-navbar">
    <AppIconButton
      size="sm"
      label="Back"
      :disabled="!ready || state.historyAt <= 0"
      @click="goHistory(tabId, -1)"
    >
      <TabIcon :shapes="shapesOf(FILES_PATHS.back)" />
    </AppIconButton>
    <AppIconButton
      size="sm"
      label="Forward"
      :disabled="!ready || state.historyAt >= state.history.length - 1"
      @click="goHistory(tabId, 1)"
    >
      <TabIcon :shapes="shapesOf(FILES_PATHS.forward)" />
    </AppIconButton>
    <AppInput
      v-model="draft"
      class="path"
      size="sm"
      aria-label="Folder path"
      spellcheck="false"
      :disabled="!ready"
      @focus="editing = true"
      @blur="onBlur"
      @keydown="onKey"
    />
    <AppIconButton
      v-if="state.clipboard"
      size="sm"
      tone="accent"
      :label="`Paste ${state.clipboard.name}`"
      :disabled="!ready"
      @click="paste(tabId)"
    >
      <TabIcon :shapes="shapesOf(FILES_PATHS.paste)" />
    </AppIconButton>
    <AppIconButton size="sm" label="Reload" :busy="state.loading" @click="load(tabId)">
      <TabIcon :shapes="shapesOf(FILES_PATHS.reload)" />
    </AppIconButton>
    <AppSwitcher
      :items="VIEWS"
      :active-id="state.view"
      placeholder="View"
      empty-text="No views."
      size="sm"
      @select="(id) => setView(tabId, id as FilesViewKind)"
    />
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
    <AppIconButton
      class="close"
      size="sm"
      icon="close"
      label="Close Files"
      @click="emit('close')"
    />
  </header>
</template>

<style scoped>
.files-navbar {
  display: flex;
  align-items: center;
  gap: 2px;
  min-height: 38px;
  padding: 0 var(--space-2);
  border-bottom: 1px solid var(--color-border-subtle);
}

.files-navbar .path {
  flex: 1;
  min-width: 80px;
  margin: 0 var(--space-1);
}

/* Close is the one control on the right side, set apart from the folder tools. */
.files-navbar .close {
  margin-left: var(--space-3);
}

.root-glyph {
  font-family: var(--font-body);
  font-size: var(--text-sm);
  font-weight: var(--weight-bold);
  line-height: 1;
}
</style>
