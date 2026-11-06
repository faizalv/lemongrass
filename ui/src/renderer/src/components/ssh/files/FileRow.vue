<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// One remote entry in the Files drawer. Its actions show on hover or focus over the row's own
// solid hover fill, covering the size and date columns, so the row keeps one height.
import { computed, nextTick, ref } from 'vue'
import { AppIconButton, AppInput, type AppInputHandle } from '@sankasten'
import TabIcon from '../../TabIcon.vue'
import { fileIconOf, folderIconOf } from '../../../fileIcons'
import { formatSize } from '../../../explorerOps'
import { formatAge } from '../../../gitTime'
import { DRAWER_ICONS } from './drawerIcons'
import { nameProblem } from './remotePath'
import type { SSHFileEntry } from '../../../../../preload/types'

const props = defineProps<{ entry: SSHFileEntry }>()

const emit = defineEmits<{
  enter: []
  download: []
  rename: [to: string]
  remove: []
}>()

const icon = computed(() =>
  props.entry.dir ? folderIconOf(props.entry.name, false) : fileIconOf(props.entry.name)
)
const modified = computed(() => new Date(props.entry.mtime).toLocaleString())
const title = computed(() => (props.entry.link ? `${props.entry.name} (link)` : props.entry.name))

const editing = ref(false)
const draft = ref('')
const input = ref<AppInputHandle | null>(null)
const problem = computed(() => (draft.value === props.entry.name ? null : nameProblem(draft.value)))

async function startRename(): Promise<void> {
  draft.value = props.entry.name
  editing.value = true
  await nextTick()
  input.value?.focus()
  input.value?.select()
}

function commit(): void {
  const to = draft.value.trim()
  editing.value = false
  if (to && to !== props.entry.name && !nameProblem(to)) emit('rename', to)
}

function onKey(event: KeyboardEvent): void {
  if (event.key === 'Enter') commit()
  else if (event.key === 'Escape') editing.value = false
}
</script>

<template>
  <div
    class="file-row"
    :class="{ dir: entry.dir, editing }"
    tabindex="-1"
    :title="title"
    @dblclick="entry.dir && !editing && emit('enter')"
  >
    <TabIcon class="icon" :shapes="icon.shapes" :style="{ color: icon.color }" />
    <template v-if="editing">
      <AppInput
        ref="input"
        v-model="draft"
        class="rename"
        size="sm"
        :aria-invalid="!!problem"
        :title="problem ?? undefined"
        aria-label="New name"
        @keydown="onKey"
        @blur="editing = false"
      />
    </template>
    <template v-else>
      <span class="name" :class="{ link: entry.link }">{{ entry.name }}</span>
      <span class="size">{{ entry.dir ? '' : formatSize(entry.size) }}</span>
      <span class="age" :title="modified">{{ formatAge(entry.mtime / 1000) }}</span>
      <span class="mode">{{ entry.mode }}</span>
      <span class="actions">
        <AppIconButton
          v-if="!entry.dir"
          size="xs"
          tone="accent"
          :label="`Download ${entry.name}`"
          @click="emit('download')"
        >
          <TabIcon :shapes="DRAWER_ICONS.download" />
        </AppIconButton>
        <AppIconButton size="xs" tone="accent" :label="`Rename ${entry.name}`" @click="startRename">
          <TabIcon :shapes="DRAWER_ICONS.rename" />
        </AppIconButton>
        <AppIconButton
          size="xs"
          tone="danger"
          icon="trash"
          confirm
          :label="`Delete ${entry.name}`"
          confirm-label="Click again to delete"
          @confirm="emit('remove')"
        />
      </span>
    </template>
  </div>
</template>

<style scoped>
.file-row {
  position: relative;
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
  outline: none;
}

.file-row.dir {
  cursor: pointer;
}

.file-row:hover,
.file-row:focus-within {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
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

.actions {
  position: absolute;
  top: 50%;
  right: var(--space-2);
  display: none;
  align-items: center;
  gap: 2px;
  padding-left: var(--space-2);
  background: var(--color-surface-2);
  transform: translateY(-50%);
}

.file-row:hover .actions,
.file-row:focus-within .actions {
  display: flex;
}

.rename {
  flex: 1;
  min-width: 0;
}

@container files-drawer (max-width: 420px) {
  .mode {
    display: none;
  }
}
</style>
