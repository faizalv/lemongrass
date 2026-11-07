<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// A server tab's editor, shown in the terminal's place while a file is in front. Files are read
// and saved through the tab's file helper; a save is refused when the server's copy moved since
// the read, and the banner then offers Reload or Overwrite.
import { computed } from 'vue'
import { AppButton, AppEmptyState, AppIconButton } from '@sankasten'
import CodeEditor from '../../CodeEditor.vue'
import EditorStrip from './EditorStrip.vue'
import { sshIcon } from '../../../icons'
import { actingAsRoot } from '../files/filesStore'
import {
  closeServerFile,
  editorChanged,
  editorStateOf,
  readServerFile,
  saveServerFile,
  serverEditorOf,
  minimizeEditor,
  showServerFile
} from './remoteFiles'

// roomForFiles: the Files button sits over the strip's right end while the layer is closed.
const props = defineProps<{ serverTabId: string; alias: string; roomForFiles: boolean }>()

const editor = computed(() => serverEditorOf(props.serverTabId, props.alias))
const file = computed(() => editor.value.files.find((f) => f.path === editor.value.active))
const name = computed(() => {
  const path = file.value?.path ?? ''
  return path.slice(path.lastIndexOf('/') + 1)
})
const root = computed(() => actingAsRoot(props.serverTabId))

function save(force = false): void {
  if (file.value) void saveServerFile(props.serverTabId, file.value.path, force)
}

function read(): void {
  if (file.value) void readServerFile(props.serverTabId, file.value.path)
}
</script>

<template>
  <div class="server-editor">
    <EditorStrip
      :files="editor.files"
      :active="editor.active"
      :room-for-files="roomForFiles"
      @minimize="minimizeEditor(serverTabId)"
      @show="(path) => showServerFile(serverTabId, path)"
      @close="(path) => closeServerFile(serverTabId, path)"
    />

    <template v-if="file">
      <div class="file-bar">
        <span class="file-path" :title="file.path">{{ file.path }}</span>
        <AppButton
          variant="primary"
          size="sm"
          :disabled="file.status !== 'ready' || !file.dirty || file.saving"
          @click="save()"
        >
          {{ file.saving ? 'Saving' : 'Save' }}
        </AppButton>
      </div>

      <div v-if="root" class="banner root" role="status">Acting as root</div>
      <div v-if="file.changed" class="banner warn" role="alert">
        <span class="banner-text">Changed on the server since you opened it.</span>
        <AppButton size="sm" @click="read">Reload</AppButton>
        <AppButton variant="danger" size="sm" :disabled="file.saving" @click="save(true)">
          Overwrite
        </AppButton>
      </div>
      <div v-if="file.notice" class="banner warn" role="alert">
        <span class="banner-text">{{ file.notice }}</span>
        <AppIconButton size="xs" icon="close" label="Dismiss" @click="file.notice = ''" />
      </div>

      <AppEmptyState
        v-if="file.status === 'notLoggedIn'"
        layout="fill"
        :icon="sshIcon"
        title="Log in in the terminal first."
        detail="The editor uses the terminal's connection."
      >
        <template #action>
          <AppButton @click="read">Retry</AppButton>
        </template>
      </AppEmptyState>
      <AppEmptyState
        v-else-if="file.status === 'binary'"
        layout="fill"
        :title="`${name} is not a text file.`"
      />
      <AppEmptyState
        v-else-if="file.status === 'tooLarge'"
        layout="fill"
        :title="`${name} is larger than 2 MB.`"
        detail="Download it from Files instead."
      />
      <AppEmptyState
        v-else-if="file.status === 'isDir'"
        layout="fill"
        :title="`${name} is a folder.`"
      />
      <AppEmptyState
        v-else-if="file.status === 'error'"
        layout="fill"
        :title="`${name} cannot be read.`"
        :detail="file.error"
      >
        <template #action>
          <AppButton @click="read">Retry</AppButton>
        </template>
      </AppEmptyState>
      <AppEmptyState
        v-else-if="file.status === 'loading'"
        layout="fill"
        loading
        :title="`Opening ${name}`"
      />
      <!-- One editor per file, so switching files keeps each one's state through restore. -->
      <CodeEditor
        v-else
        :key="file.path"
        :content="file.content"
        :path="file.path"
        :revision="file.revision"
        :restore="editorStateOf(serverTabId, file.path)"
        autofocus
        @change="(state) => file && editorChanged(serverTabId, file.path, state)"
        @save="save()"
      />
    </template>
  </div>
</template>

<style scoped>
.server-editor {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--color-surface-0);
  font-family: var(--font-body);
}

.file-bar {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 34px;
  padding: 0 var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
}

.file-path {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.banner {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 34px;
  padding: 0 var(--space-3);
  border-bottom: 1px solid var(--color-border-subtle);
  font-size: var(--text-xs);
}

.banner.root {
  min-height: 28px;
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
</style>
