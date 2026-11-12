<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The layer's occasional status at the bottom left: a spinner with what a slow operation is doing
// and one line per running transfer, Cancel showing on hover. Nothing renders otherwise, and a
// failed transfer reports in the banner instead.
import { AppIcon, AppIconButton } from '@sankasten'
import { formatSize } from '../../../explorerOps'
import type { FileTransfer } from './filesStore'

defineProps<{ slow: string; transfers: FileTransfer[] }>()

const emit = defineEmits<{ cancel: [id: string] }>()

function transferText(t: FileTransfer): string {
  const verb = { upload: 'Uploading', download: 'Downloading', relay: 'Copying' }[t.direction]
  const what = t.direction === 'relay' ? `${t.name} from ${t.from}` : t.name
  if (t.total > 0) return `${verb} ${what} ${Math.min(100, Math.round((t.done / t.total) * 100))}%`
  return t.done > 0 ? `${verb} ${what} ${formatSize(t.done)}` : `${verb} ${what}`
}
</script>

<template>
  <div v-if="slow || transfers.length" class="files-status" role="status">
    <div v-if="slow" class="line">
      <AppIcon name="spinner" class="spin" />
      <span class="text">{{ slow }}</span>
    </div>
    <div v-for="t in transfers" :key="t.id" class="line transfer">
      <AppIcon name="spinner" class="spin" />
      <span class="text">{{ transferText(t) }}</span>
      <AppIconButton
        class="cancel"
        size="xs"
        tone="danger"
        icon="close"
        :label="`Cancel ${t.name}`"
        @click="emit('cancel', t.id)"
      />
    </div>
  </div>
</template>

<style scoped>
.files-status {
  position: absolute;
  left: var(--space-2);
  bottom: var(--space-2);
  z-index: 2;
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-width: calc(100% - var(--space-4));
  max-height: 40%;
  overflow-y: auto;
  padding: var(--space-1) var(--space-2);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-md);
  background: var(--layer-float);
  box-shadow: var(--shadow-md);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  color: var(--color-fg-secondary);
}

.line {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 22px;
  min-width: 0;
}

.spin {
  flex-shrink: 0;
  color: var(--color-jade);
  animation: spin 1s linear infinite;
}

.text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.cancel {
  visibility: hidden;
}

.transfer:hover .cancel,
.transfer:focus-within .cancel {
  visibility: visible;
}

@media (prefers-reduced-motion: reduce) {
  .spin {
    animation: none;
  }
}
</style>
