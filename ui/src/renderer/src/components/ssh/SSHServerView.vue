<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// A server tab: a terminal logged in to one server, with the Files layer over its right side and
// the editor in the terminal's place while a file is in front. The ssh command line comes from
// main, which owns the ControlMaster socket folder; the terminal itself is an ordinary shell
// registry session, so it keeps running while the tab is hidden, moved or behind the editor.
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { AppButton, AppEdgeTab, AppEmptyState } from '@sankasten'
import ShellView from '../ShellView.vue'
import TabIcon from '../TabIcon.vue'
import FilesLayer from './files/FilesLayer.vue'
import ServerEditor from './editor/ServerEditor.vue'
import { expandEditor, serverEditorOf, serverHasDirtyFiles } from './editor/remoteFiles'
import { FILES_PATHS, shapesOf } from './files/drawerIcons'
import { filesOf } from './files/filesStore'
import { describeError } from '../../vaultSession'
import { sshIcon } from '../../icons'
import { focusShell, type ShellSpec } from '../../shellRegistry'

const props = defineProps<{
  tabId: string
  alias: string
  active: boolean
}>()

const spec = ref<ShellSpec | null>(null)
const error = ref('')
const files = computed(() => filesOf(props.tabId, props.alias))
const editor = computed(() => serverEditorOf(props.tabId, props.alias))
const editing = computed(() => !editor.value.minimized && editor.value.files.length > 0)
const minimized = computed(() => editor.value.minimized && editor.value.files.length > 0)
const unsaved = computed(() => serverHasDirtyFiles(props.tabId))
const bookmarkLabel = computed(() => {
  const count = editor.value.files.length
  const open = count === 1 ? '1 file open' : `${count} files open`
  return unsaved.value ? `Show editor, ${open}, unsaved changes` : `Show editor, ${open}`
})
const terminalSlot = ref<HTMLElement | null>(null)

// Keys must never reach the hidden terminal, since they would run on the server unseen: focus
// inside it is dropped when the editor takes its place (the slot is also inert meanwhile), and
// given back when the terminal returns to the front of the shown tab.
watch(editing, async (now) => {
  if (now) {
    const focused = document.activeElement
    if (focused instanceof HTMLElement && terminalSlot.value?.contains(focused)) focused.blur()
    return
  }
  await nextTick()
  if (props.active) focusShell(props.tabId)
})

onMounted(async () => {
  try {
    const args = await window.api.ssh.terminalArgs(props.alias)
    spec.value = { id: props.tabId, command: 'ssh', args }
  } catch (err) {
    error.value = describeError(err)
  }
})
</script>

<template>
  <div v-if="spec" class="server-view">
    <!-- Hidden, not unmounted, behind the editor: it stays laid out at its size, so it never
         refits or redraws, inactive like a background tab, and inert so no key reaches it. -->
    <div ref="terminalSlot" class="terminal-slot" :class="{ behind: editing }" :inert="editing">
      <ShellView :spec="spec" :active="active && !editing" />
    </div>
    <div v-if="editing" class="editor-slot" :class="{ beside: files.open }">
      <ServerEditor :server-tab-id="tabId" :alias="alias" :room-for-files="!files.open" />
    </div>
    <!-- The minimized editor, parked on the Files layer's left edge. -->
    <AppEdgeTab
      v-if="minimized && files.open"
      class="editor-bookmark"
      :label="bookmarkLabel"
      :count="editor.files.length"
      :dot="unsaved"
      @click="expandEditor(tabId)"
    >
      <template #icon><TabIcon :shapes="shapesOf(FILES_PATHS.open)" /></template>
    </AppEdgeTab>
    <FilesLayer v-if="files.open" :tab-id="tabId" :alias="alias" @close="files.open = false" />
    <!-- Over terminal text, so it keeps a solid fill; the layer's own Close replaces it while open. -->
    <AppButton v-else class="files-toggle" variant="primary" size="sm" @click="files.open = true">
      <TabIcon :shapes="shapesOf(FILES_PATHS.files)" />
      Files
      <span
        v-if="minimized && unsaved"
        class="files-unsaved"
        title="A minimized file has unsaved changes"
      />
    </AppButton>
  </div>
  <AppEmptyState
    v-else-if="error"
    layout="fill"
    :icon="sshIcon"
    :title="`${alias} cannot open.`"
    :detail="error"
  />
</template>

<style scoped>
.server-view {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.terminal-slot {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.terminal-slot.behind {
  visibility: hidden;
}

/* The editor takes the terminal's place; with the Files layer open its right edge meets the
   layer's left edge (the layer is max(320px, 45%) wide), so the two read as one panel. */
.editor-slot {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: flex;
  flex-direction: column;
}

.editor-slot.beside {
  right: max(320px, 45%);
}

/* Against the layer's left edge, which is max(320px, 45%) from the tab's right. */
.server-view .editor-bookmark {
  position: absolute;
  top: 48px;
  right: max(320px, 45%);
  z-index: 3;
}

.server-view .files-toggle {
  position: absolute;
  top: 5px;
  right: var(--space-4);
  z-index: 3;
  gap: var(--space-1);
}

/* Dark on the button's amber fill. */
.files-unsaved {
  width: 6px;
  height: 6px;
  margin-left: 2px;
  border-radius: 50%;
  background: currentColor;
}
</style>
