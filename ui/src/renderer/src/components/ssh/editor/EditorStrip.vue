<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The strip atop a server tab's editor: one small tab per open file, and at the right end a
// chevron pointing toward the Files layer that folds the editor into a bookmark on it. A dirty
// file's close mark asks for a second click, like a dirty workspace tab.
import { AppIconButton, AppTab, vWheelScrollX } from '@sankasten'
import TabIcon from '../../TabIcon.vue'
import { fileIconOf } from '../../../fileIcons'
import type { RemoteFile } from './remoteFiles'

// roomForFiles keeps the strip's right end clear of the Files button, shown while the layer is
// closed.
defineProps<{ files: RemoteFile[]; active: string | null; roomForFiles: boolean }>()

const emit = defineEmits<{
  minimize: []
  show: [path: string]
  close: [path: string]
}>()

function nameOf(path: string): string {
  return path.slice(path.lastIndexOf('/') + 1)
}
</script>

<template>
  <div class="editor-strip" :class="{ 'room-for-files': roomForFiles }">
    <div v-wheel-scroll-x class="strip-tabs sk-scroll-slim" role="tablist" aria-label="Open files">
      <AppTab
        v-for="file in files"
        :key="file.path"
        :label="nameOf(file.path)"
        :title="file.path"
        :active="active === file.path"
        :dirty="file.dirty"
        :close-label="`Close ${nameOf(file.path)}`"
        @select="emit('show', file.path)"
        @close="emit('close', file.path)"
      >
        <template #icon>
          <TabIcon
            :shapes="fileIconOf(nameOf(file.path)).shapes"
            :style="{ color: fileIconOf(nameOf(file.path)).color }"
          />
        </template>
      </AppTab>
    </div>
    <AppIconButton
      class="minimize"
      size="sm"
      icon="chevronRight"
      label="Minimize editor"
      @click="emit('minimize')"
    />
  </div>
</template>

<style scoped>
.editor-strip {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  min-height: 34px;
  padding: 0 var(--space-2);
  border-bottom: 1px solid var(--color-border-subtle);
  background: var(--color-surface-1);
  font-family: var(--font-body);
}

.editor-strip.room-for-files {
  padding-right: 96px;
}

.strip-tabs {
  flex: 1;
  min-width: 0;
  align-self: stretch;
  display: flex;
  align-items: stretch;
  gap: 2px;
  overflow-x: auto;
}

.minimize {
  flex-shrink: 0;
}
</style>
