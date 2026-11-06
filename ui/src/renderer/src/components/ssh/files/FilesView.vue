<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The shown folder in one of three views: Detailed and Basic rows, or a Tile grid of large icons.
// Click selects, double-click enters a folder, and a right click asks the layer for the entry's
// menu, or the empty space's menu when it lands outside every entry.
import TabIcon from '../../TabIcon.vue'
import FileRow from './FileRow.vue'
import { fileIconOf, folderIconOf } from '../../../fileIcons'
import type { FilesViewKind } from './filesStore'
import type { SSHFileEntry } from '../../../../../preload/types'

defineProps<{
  entries: SSHFileEntry[]
  view: FilesViewKind
  selected: string | null
}>()

const emit = defineEmits<{
  select: [name: string | null]
  open: [entry: SSHFileEntry]
  menu: [event: MouseEvent, entry: SSHFileEntry | null]
}>()

function iconOf(entry: SSHFileEntry): ReturnType<typeof fileIconOf> {
  return entry.dir ? folderIconOf(entry.name, false) : fileIconOf(entry.name)
}

function onMenu(event: MouseEvent, entry: SSHFileEntry | null): void {
  event.preventDefault()
  event.stopPropagation()
  emit('select', entry?.name ?? null)
  emit('menu', event, entry)
}
</script>

<template>
  <div
    class="files-view"
    :class="view"
    role="list"
    @click.self="emit('select', null)"
    @contextmenu="onMenu($event, null)"
  >
    <template v-if="view === 'tile'">
      <div
        v-for="entry in entries"
        :key="entry.name"
        class="tile"
        :class="{ selected: selected === entry.name }"
        role="listitem"
        :title="entry.link ? `${entry.name} (link)` : entry.name"
        @click="emit('select', entry.name)"
        @dblclick="entry.dir && emit('open', entry)"
        @contextmenu="onMenu($event, entry)"
      >
        <TabIcon
          :shapes="iconOf(entry).shapes"
          :size="32"
          :style="{ color: iconOf(entry).color }"
        />
        <span class="tile-name" :class="{ link: entry.link }">{{ entry.name }}</span>
      </div>
    </template>
    <template v-else>
      <FileRow
        v-for="entry in entries"
        :key="entry.name"
        role="listitem"
        :entry="entry"
        :detailed="view === 'detailed'"
        :selected="selected === entry.name"
        @click="emit('select', entry.name)"
        @dblclick="entry.dir && emit('open', entry)"
        @contextmenu="onMenu($event, entry)"
      />
    </template>
  </div>
</template>

<style scoped>
.files-view {
  flex: 1;
  min-height: 0;
  padding: var(--space-2);
  font-family: var(--font-body);
}

.files-view.detailed,
.files-view.basic {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.files-view.tile {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(88px, 1fr));
  align-content: start;
  gap: var(--space-1);
}

.tile {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-1);
  min-width: 0;
  padding: var(--space-2) var(--space-1);
  border-radius: var(--radius-md);
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
  cursor: default;
  user-select: none;
}

.tile:hover {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.tile.selected {
  background: var(--color-amber-muted);
  color: var(--color-fg-accent);
}

.tile-name {
  max-width: 100%;
  overflow: hidden;
  text-align: center;
  overflow-wrap: anywhere;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.tile-name.link {
  font-style: italic;
}
</style>
