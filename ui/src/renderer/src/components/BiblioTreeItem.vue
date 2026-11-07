<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed } from 'vue'
import { AppIconButton } from '@sankasten'
import TabIcon from './TabIcon.vue'
import MatchName from './treeSearch/MatchName.vue'
import { archiveIcon, biblioDirShapes, biblioFileShapes } from '../biblioIcons'
import { biblioLabel } from '../biblioSearch'
import type { MatchRange } from '../treeSearch'
import type { BiblioNode } from '../../../preload/types'

// The expanded folders live in the panel, by path, so a search can open the folders above its
// matches and the tree keeps its shape across a switch to the Files view and back.
const props = defineProps<{
  node: BiblioNode
  selectedPath: string | null
  depth: number
  expandedPaths: Record<string, true>
  // An active search: the matched rows with the ranges to highlight, and its keyboard cursor.
  matches?: Map<string, MatchRange[]> | null
  cursorPath?: string | null
  // Categories the search found nothing in, which show a line saying so.
  noMatch?: Set<string> | null
}>()

const ranges = computed((): MatchRange[] | undefined => props.matches?.get(props.node.path))

const isCursor = computed((): boolean => !!props.cursorPath && props.cursorPath === props.node.path)

const emit = defineEmits<{
  select: [path: string]
  toggle: [path: string]
  expand: [path: string]
  'add-scratchpad': []
  'add-scratchpad-file': [folderPath: string]
  'archive-scratchpad': [taskPath: string]
}>()

const expanded = computed((): boolean => !!props.expandedPaths[props.node.path])

const isArchive = computed((): boolean => props.node.path === 'scratchpad/archive')

const isTopLevelScratchpad = computed(
  (): boolean => props.depth === 0 && props.node.name === 'scratchpad'
)

// Covers the top-level scratchpad row and every folder nested under scratchpad/, except the archive.
const showAddButton = computed(
  (): boolean =>
    props.node.type === 'dir' &&
    props.node.path !== 'scratchpad/archive' &&
    !props.node.path.startsWith('scratchpad/archive/') &&
    (isTopLevelScratchpad.value ||
      props.node.path === 'scratchpad' ||
      props.node.path.startsWith('scratchpad/'))
)

function onAddClick(): void {
  emit('expand', props.node.path)
  if (isTopLevelScratchpad.value) {
    emit('add-scratchpad')
  } else {
    emit('add-scratchpad-file', props.node.path)
  }
}

// A task folder directly under scratchpad/ (e.g. "scratchpad/some-task"), excluding
// the archive folder itself -- that's the one row this button is offered on.
const isArchivableScratchpadTask = computed((): boolean => {
  if (props.node.type !== 'dir') return false
  const segments = props.node.path.split('/')
  return segments.length === 2 && segments[0] === 'scratchpad' && segments[1] !== 'archive'
})
</script>

<template>
  <button
    v-if="node.type === 'file'"
    class="tree-item file"
    :class="{ active: node.path === selectedPath, 'search-cursor': isCursor }"
    :style="{ paddingLeft: `${depth * 14 + 10}px` }"
    @click="emit('select', node.path)"
  >
    <span class="indent" />
    <TabIcon class="icon" :shapes="biblioFileShapes(node.name)" />
    <MatchName class="label" :text="biblioLabel(node)" :ranges="ranges" />
  </button>

  <div
    v-else
    class="dir-row"
    :class="{ 'archive-row': isArchive, 'search-cursor': isCursor }"
    :style="isArchive ? { marginLeft: `${depth * 14}px`, width: 'auto' } : undefined"
  >
    <button
      class="tree-item tree-item-main"
      :style="isArchive ? undefined : { paddingLeft: `${depth * 14 + 10}px` }"
      @click="emit('toggle', node.path)"
    >
      <svg
        class="indent chevron"
        :class="{ expanded }"
        width="9"
        height="9"
        viewBox="0 0 9 9"
        fill="none"
        stroke="currentColor"
        stroke-width="1.4"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M2 1.5 6.5 4.5 2 7.5" />
      </svg>

      <TabIcon class="icon" :shapes="biblioDirShapes(node.path)" />

      <MatchName class="label" :text="node.name" :ranges="ranges" />
    </button>
    <AppIconButton
      v-if="isArchivableScratchpadTask"
      class="tree-item-add"
      size="xs"
      tone="accent"
      label="Archive"
      @click.stop="emit('archive-scratchpad', node.path)"
    >
      <TabIcon :shapes="archiveIcon" />
    </AppIconButton>
    <AppIconButton
      v-if="showAddButton"
      class="tree-item-add"
      size="xs"
      tone="accent"
      icon="plus"
      :label="isTopLevelScratchpad ? 'Add scratchpad' : 'Add file'"
      @click.stop="onAddClick"
    />
  </div>

  <p
    v-if="noMatch?.has(node.path)"
    class="no-match"
    :style="{ paddingLeft: `${(depth + 1) * 14 + 10}px` }"
  >
    No match
  </p>

  <template v-if="node.children && expanded">
    <BiblioTreeItem
      v-for="child in node.children"
      :key="child.path"
      :node="child"
      :selected-path="selectedPath"
      :depth="depth + 1"
      :expanded-paths="expandedPaths"
      :matches="matches"
      :cursor-path="cursorPath"
      :no-match="noMatch"
      @select="(path) => emit('select', path)"
      @toggle="(path) => emit('toggle', path)"
      @expand="(path) => emit('expand', path)"
      @add-scratchpad="emit('add-scratchpad')"
      @add-scratchpad-file="(folderPath) => emit('add-scratchpad-file', folderPath)"
      @archive-scratchpad="(taskPath) => emit('archive-scratchpad', taskPath)"
    />
  </template>
</template>

<style scoped>
.archive-row {
  margin-top: var(--space-1);
  margin-bottom: var(--space-1);
  margin-right: var(--space-2);
  background: var(--color-surface-2);
  border-radius: var(--radius-md);
  transition: background var(--duration-fast) var(--ease-out);
}

.archive-row:hover {
  background: var(--color-surface-3);
}

.archive-row .tree-item-main {
  padding-left: var(--space-3);
  color: var(--color-fg-primary);
  font-weight: var(--weight-semibold);
}

.archive-row .tree-item:hover {
  background: transparent;
}

.tree-item {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-width: 0;
  text-align: left;
  padding-top: var(--space-1);
  padding-bottom: var(--space-1);
  padding-right: var(--space-3);
  background: transparent;
  border: none;
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  cursor: pointer;
  border-radius: var(--radius-md);
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.tree-item:hover {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.tree-item.file.active {
  background: var(--color-amber-muted);
  color: var(--color-fg-accent);
  font-weight: var(--weight-medium);
}

.tree-item.search-cursor,
.dir-row.search-cursor {
  box-shadow: inset 0 0 0 1px var(--color-border-strong);
  border-radius: var(--radius-md);
}

.no-match {
  margin: 0;
  padding-top: var(--space-1);
  padding-bottom: var(--space-1);
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.dir-row {
  display: flex;
  align-items: center;
  width: 100%;
  min-width: 0;
}

.tree-item-main {
  flex: 1;
  color: var(--color-fg-secondary);
  font-weight: var(--weight-medium);
}

.tree-item-add {
  margin-right: var(--space-2);
}

.indent {
  flex-shrink: 0;
  width: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.chevron {
  color: var(--color-fg-muted);
  transition: transform var(--duration-fast) var(--ease-out);
}

.chevron.expanded {
  transform: rotate(90deg);
}

.icon {
  flex-shrink: 0;
  color: var(--color-fg-muted);
}

.tree-item-main .icon {
  color: var(--color-fg-secondary);
}

.label {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
