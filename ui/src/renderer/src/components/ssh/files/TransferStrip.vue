<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The drawer's foot: one line per transfer with its progress. A running transfer cancels, a
// failed one stays with its reason until dismissed, and a finished one clears itself.
import { AppIconButton } from '@sankasten'
import TabIcon from '../../TabIcon.vue'
import { formatSize } from '../../../explorerOps'
import { DRAWER_ICONS } from './drawerIcons'
import type { FileTransfer } from './filesStore'

defineProps<{ transfers: FileTransfer[] }>()

const emit = defineEmits<{
  cancel: [id: string]
  dismiss: [id: string]
}>()

function percent(t: FileTransfer): number {
  if (t.state === 'done') return 100
  return t.total > 0 ? Math.min(100, Math.round((t.done / t.total) * 100)) : 0
}

function status(t: FileTransfer): string {
  if (t.state === 'done') return 'Done'
  if (t.state === 'failed') return t.error
  if (!t.total) return formatSize(t.done)
  return `${formatSize(t.done)} of ${formatSize(t.total)}`
}
</script>

<template>
  <div v-if="transfers.length" class="transfer-strip">
    <div v-for="t in transfers" :key="t.id" class="transfer" :class="t.state">
      <TabIcon
        class="direction"
        :shapes="t.direction === 'upload' ? DRAWER_ICONS.upload : DRAWER_ICONS.download"
        :aria-label="t.direction === 'upload' ? 'Upload' : 'Download'"
      />
      <div class="body">
        <div class="line">
          <span class="name">{{ t.name }}</span>
          <span class="status" :title="status(t)">{{ status(t) }}</span>
        </div>
        <div
          v-if="t.state !== 'failed'"
          class="track"
          role="progressbar"
          :aria-label="`${t.name} transfer`"
          aria-valuemin="0"
          aria-valuemax="100"
          :aria-valuenow="percent(t)"
        >
          <div class="fill" :style="{ width: `${percent(t)}%` }" />
        </div>
      </div>
      <AppIconButton
        v-if="t.state === 'running'"
        size="xs"
        tone="danger"
        icon="close"
        :label="`Cancel ${t.name}`"
        @click="emit('cancel', t.id)"
      />
      <AppIconButton
        v-else-if="t.state === 'failed'"
        size="xs"
        icon="close"
        :label="`Dismiss ${t.name}`"
        @click="emit('dismiss', t.id)"
      />
    </div>
  </div>
</template>

<style scoped>
.transfer-strip {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  max-height: 150px;
  overflow-y: auto;
  padding: var(--space-2) var(--space-3);
  border-top: 1px solid var(--color-border-subtle);
  font-family: var(--font-body);
  font-size: var(--text-xs);
}

.transfer {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 26px;
  color: var(--color-fg-secondary);
}

.direction {
  flex-shrink: 0;
  color: var(--color-fg-muted);
}

.body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.line {
  display: flex;
  gap: var(--space-2);
  min-width: 0;
}

.name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-primary);
}

.status {
  flex-shrink: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-muted);
  font-variant-numeric: tabular-nums;
}

.failed .status {
  color: var(--color-error);
}

.done .status {
  color: var(--color-jade);
}

.track {
  height: 3px;
  border-radius: var(--radius-pill);
  background: var(--color-jade-muted);
  overflow: hidden;
}

.fill {
  height: 100%;
  background: var(--color-jade);
  transition: width var(--duration-fast) var(--ease-out);
}
</style>
