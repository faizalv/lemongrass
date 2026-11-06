<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed } from 'vue'
import { AppButton } from '@sankasten'
import { formatSize } from '../../explorerOps'
import type { ResterClientPickedFile } from '../../../../preload/types'

const props = defineProps<{
  path: string
  size: number
}>()

const emit = defineEmits<{ picked: [file: ResterClientPickedFile] }>()

const name = computed(() => props.path.split(/[\\/]/).pop() ?? props.path)

async function choose(): Promise<void> {
  const file = await window.api.resterClient.pickFile()
  if (file) emit('picked', file)
}
</script>

<!-- Main reads the file when the request is sent, so only its path and its size at pick time are kept. -->
<template>
  <div class="file-pick">
    <AppButton class="choose" size="sm" @click="choose">{{
      path ? 'Change' : 'Choose file'
    }}</AppButton>
    <template v-if="path">
      <span class="file-name" :title="path">{{ name }}</span>
      <span v-if="size" class="file-size">{{ formatSize(size) }}</span>
    </template>
    <span v-else class="file-none">No file chosen</span>
  </div>
</template>

<style scoped>
.file-pick {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
}

.choose {
  flex-shrink: 0;
}

.file-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-primary);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
}

.file-size,
.file-none {
  flex-shrink: 0;
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}
</style>
