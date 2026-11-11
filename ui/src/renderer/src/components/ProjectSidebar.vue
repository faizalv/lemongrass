<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { AppEmptyState, AppIcon, AppIconButton, AppSegmented } from '@sankasten'
import BiblioPanel from './BiblioPanel.vue'
import FileExplorer from './explorer/FileExplorer.vue'
import GitPanel from './GitPanel.vue'
import SidebarRail from './SidebarRail.vue'
import WorkgroupPanel from './WorkgroupPanel.vue'
import SSHPanel from './ssh/SSHPanel.vue'
import {
  loadSidebarSettings,
  setSidebarView,
  sidebarSettings,
  setFoldHeight,
  toggleSidebarPanel,
  type SidebarPanel,
  type SidebarView
} from '../explorerSettings'
import type { BiblioTree } from '../../../preload/types'
import type { ProjectRef } from '../workspace'

defineProps<{
  project: ProjectRef | undefined
  tree: BiblioTree | null
}>()

const emit = defineEmits<{
  refreshBiblio: []
}>()

const VIEWS: { value: SidebarView; label: string }[] = [
  { value: 'biblio', label: 'Biblio' },
  { value: 'files', label: 'Files' }
]

const PANEL_NAMES: Record<SidebarPanel, string> = {
  git: 'Git'
}

onMounted(() => void loadSidebarSettings())

// The column stays nested in the sidebar with the width on the column itself, so the sidebar
// takes its natural width from it. As a sibling of the sidebar it shows a dead gap beside the tree
// under electron-vite dev.
const columnWidth = ref(240)
let dragStartX = 0
let dragStartWidth = 0

function onGutterDown(event: PointerEvent): void {
  dragStartX = event.clientX
  dragStartWidth = columnWidth.value
  window.addEventListener('pointermove', onGutterMove)
  window.addEventListener('pointerup', onGutterUp)
}

function onGutterMove(event: PointerEvent): void {
  const delta = event.clientX - dragStartX
  columnWidth.value = Math.min(480, Math.max(180, dragStartWidth + delta))
}

function onGutterUp(): void {
  window.removeEventListener('pointermove', onGutterMove)
  window.removeEventListener('pointerup', onGutterUp)
}

// The fold under the view takes the height it is given and both sides scroll inside, so neither
// can push the other out of the column. The height is saved on release, not on every move.
const FOLD_MIN = 96
const VIEW_MIN = 120
const columnRef = ref<HTMLElement | null>(null)
let dragStartY = 0
let dragStartHeight = 0

function onFoldDown(event: PointerEvent): void {
  dragStartY = event.clientY
  dragStartHeight = sidebarSettings.foldHeight
  window.addEventListener('pointermove', onFoldMove)
  window.addEventListener('pointerup', onFoldUp)
}

function onFoldMove(event: PointerEvent): void {
  const max = Math.max(FOLD_MIN, (columnRef.value?.clientHeight ?? 0) - VIEW_MIN)
  const next = dragStartHeight - (event.clientY - dragStartY)
  sidebarSettings.foldHeight = Math.min(max, Math.max(FOLD_MIN, next))
}

function onFoldUp(): void {
  window.removeEventListener('pointermove', onFoldMove)
  window.removeEventListener('pointerup', onFoldUp)
  setFoldHeight(sidebarSettings.foldHeight)
}
</script>

<template>
  <div class="sidebar">
    <SidebarRail v-if="project" :project="project" />
    <div v-show="!sidebarSettings.hidden" class="sidebar-main">
      <div v-if="project" class="sidebar-pane">
        <div ref="columnRef" class="sidebar-column" :style="{ width: `${columnWidth}px` }">
          <div class="pane-view">
            <template v-if="sidebarSettings.pane === 'files'">
              <AppSegmented
                class="view-switch"
                block
                aria-label="Sidebar view"
                :options="VIEWS"
                :model-value="sidebarSettings.view"
                @update:model-value="setSidebarView"
              />
              <BiblioPanel
                v-if="sidebarSettings.view === 'biblio'"
                :tree="tree"
                :project="project"
                @refresh="emit('refreshBiblio')"
              />
              <FileExplorer v-else :project="project" />
            </template>
            <template v-else>
              <template v-if="sidebarSettings.pane === 'workgroup'">
                <div class="column-head">
                  <span class="column-title">Workgroup</span>
                </div>
                <WorkgroupPanel :project="project" />
              </template>
              <!-- The Servers | Keys switch heads the SSH view, as Biblio | Files heads the tree. -->
              <SSHPanel v-else :project="project" />
            </template>
          </div>

          <template v-if="sidebarSettings.panel">
            <div class="fold-gutter" @pointerdown="onFoldDown" />
            <div class="panel-fold" :style="{ height: `${sidebarSettings.foldHeight}px` }">
              <div class="column-head">
                <span class="column-title">{{ PANEL_NAMES[sidebarSettings.panel] }}</span>
                <AppIconButton
                  size="sm"
                  :label="`Minimize ${PANEL_NAMES[sidebarSettings.panel]}`"
                  @click="sidebarSettings.panel && toggleSidebarPanel(sidebarSettings.panel)"
                >
                  <AppIcon name="minus" />
                </AppIconButton>
              </div>
              <GitPanel :project="project" />
            </div>
          </template>
        </div>

        <div class="sidebar-gutter" @pointerdown="onGutterDown" />
      </div>
      <AppEmptyState v-else layout="fill" title="No project selected." />
    </div>
  </div>
</template>

<style scoped>
.sidebar {
  flex-shrink: 0;
  display: flex;
  background: var(--color-surface-1);
  overflow: hidden;
}

.sidebar-main {
  flex: 1;
  min-height: 0;
  display: flex;
}

.sidebar-pane {
  flex-shrink: 0;
  min-height: 0;
  display: flex;
}

.sidebar-column {
  flex-shrink: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--color-surface-1);
}

.pane-view {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.pane-view .view-switch {
  margin: var(--space-3) var(--space-2) 0;
}

/* One header shape for a view and for the fold, so every panel gets its name without drawing its
   own; the fold's header adds the minimize dash. */
.column-head {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: var(--control-h-sm);
  padding: var(--space-1) var(--space-2) 0 var(--space-3);
}

.column-title {
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
}

/* Shrinks below its dragged height when the window does, down to the same floor as the drag. */
.panel-fold {
  flex-shrink: 1;
  display: flex;
  flex-direction: column;
  min-height: 96px;
}

.fold-gutter {
  flex-shrink: 0;
  height: 4px;
  cursor: row-resize;
  background: var(--color-border-subtle);
  transition: background var(--duration-fast) var(--ease-out);
}

.sidebar-gutter {
  flex-shrink: 0;
  width: 4px;
  cursor: col-resize;
  background: var(--color-border-subtle);
  transition: background var(--duration-fast) var(--ease-out);
}

.fold-gutter:hover,
.fold-gutter:active,
.sidebar-gutter:hover,
.sidebar-gutter:active {
  background: var(--color-amber);
}
</style>
