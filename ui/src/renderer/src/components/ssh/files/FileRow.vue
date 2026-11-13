<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// One remote entry as a row: the Detailed view shows size, modified time and muted permissions,
// the Basic view only the icon and name. Actions live in the context menu.
import { computed } from 'vue'
import TabIcon from '../../TabIcon.vue'
import { fileIconOf, folderIconOf } from '../../../fileIcons'
import { formatSize } from '../../../explorerOps'
import { formatAge } from '../../../gitTime'
import type { SSHFileEntry } from '../../../../../preload/types'

const props = defineProps<{ entry: SSHFileEntry; detailed: boolean; selected: boolean }>()

const icon = computed(() =>
  props.entry.dir ? folderIconOf(props.entry.name, false) : fileIconOf(props.entry.name)
)
const modified = computed(() => new Date(props.entry.mtime).toLocaleString())
const title = computed(() => (props.entry.link ? `${props.entry.name} (link)` : props.entry.name))
</script>

<template>
  <div class="file-row" :class="{ selected, dir: entry.dir }" :title="title">
    <TabIcon class="icon" :shapes="icon.shapes" :style="{ color: icon.color }" />
    <span class="name" :class="{ link: entry.link }">{{ entry.name }}</span>
    <template v-if="detailed">
      <span class="size">{{ entry.dir ? '' : formatSize(entry.size) }}</span>
      <span class="age" :title="modified">{{ formatAge(entry.mtime / 1000) }}</span>
      <span class="mode">{{ entry.mode }}</span>
    </template>
  </div>
</template>

<style scoped>
.file-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 26px;
  padding: 0 var(--space-3);
  border-radius: var(--radius-md);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  cursor: default;
  user-select: none;
}

.file-row:hover {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.file-row.selected {
  /* Muted text reads as secondary on this tint, keeping it above its contrast floor. */
  --color-fg-muted: var(--color-fg-secondary);
  background: var(--color-amber-muted);
  color: var(--color-fg-accent);
}

.icon {
  flex-shrink: 0;
}

.name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.name.link {
  font-style: italic;
}

.size,
.age {
  flex-shrink: 0;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.size {
  width: 58px;
}

.age {
  width: 34px;
}

.mode {
  flex-shrink: 0;
  width: 78px;
  color: var(--color-fg-muted);
  font-variant-numeric: tabular-nums;
}

@container files-layer (max-width: 420px) {
  .mode {
    display: none;
  }
}
</style>
