<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { AppEmptyState, AppIcon, AppIconButton, AppSegmented } from '@sankasten'
import BiblioPanel from './BiblioPanel.vue'
import FileExplorer from './explorer/FileExplorer.vue'
import GitPanel from './GitPanel.vue'
import GitPanelButton from './GitPanelButton.vue'
import WorkgroupPanel from './WorkgroupPanel.vue'
import WorkgroupPanelButton from './WorkgroupPanelButton.vue'
import {
  loadSidebarSettings,
  setSidebarView,
  sidebarSettings,
  toggleSidebarPanel,
  type SidebarPanel,
  type SidebarView
} from '../explorerSettings'
import type { BiblioTree } from '../../../preload/types'
import type { ProjectRef } from '../workspace'

defineProps<{
  collapsed: boolean
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
  workgroup: 'Workgroup',
  git: 'Git'
}

onMounted(() => void loadSidebarSettings())

// The tree column stays nested in the sidebar with the width on the column itself, so the sidebar
// takes its natural width from it. As a sibling of the sidebar it shows a dead gap beside the tree
// under electron-vite dev.
const treeWidth = ref(240)
let dragStartX = 0
let dragStartWidth = 0

function onGutterDown(event: PointerEvent): void {
  dragStartX = event.clientX
  dragStartWidth = treeWidth.value
  window.addEventListener('pointermove', onGutterMove)
  window.addEventListener('pointerup', onGutterUp)
}

function onGutterMove(event: PointerEvent): void {
  const delta = event.clientX - dragStartX
  treeWidth.value = Math.min(480, Math.max(180, dragStartWidth + delta))
}

function onGutterUp(): void {
  window.removeEventListener('pointermove', onGutterMove)
  window.removeEventListener('pointerup', onGutterUp)
}
</script>

<template>
  <div class="sidebar" :class="{ collapsed }">
    <div class="sidebar-main">
      <div v-if="project" class="sidebar-pane">
        <div class="sidebar-tree" :style="{ width: `${treeWidth}px` }">
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

          <div v-if="sidebarSettings.panel" class="panel-fold">
            <div class="panel-head">
              <span class="panel-title">{{ PANEL_NAMES[sidebarSettings.panel] }}</span>
              <AppIconButton
                size="sm"
                :label="`Minimize ${PANEL_NAMES[sidebarSettings.panel]}`"
                @click="sidebarSettings.panel && toggleSidebarPanel(sidebarSettings.panel)"
              >
                <AppIcon name="minus" />
              </AppIconButton>
            </div>
            <WorkgroupPanel v-if="sidebarSettings.panel === 'workgroup'" :project="project" />
            <GitPanel v-else :project="project" />
          </div>
          <div class="panel-bar" role="toolbar" aria-label="Panels">
            <WorkgroupPanelButton
              :project="project"
              :open="sidebarSettings.panel === 'workgroup'"
              @toggle="toggleSidebarPanel('workgroup')"
            />
            <GitPanelButton
              :project="project"
              :open="sidebarSettings.panel === 'git'"
              @toggle="toggleSidebarPanel('git')"
            />
          </div>
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
  flex-direction: column;
  background: var(--color-surface-1);
  overflow: hidden;
  transition: width var(--duration-base) var(--ease-out);
}

.sidebar.collapsed {
  width: 0;
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

/* The open panel keeps the height it had as an inline fold, so the tree above keeps the rest;
   the icon row stays at the bottom whether a panel is open or not. */
.panel-fold {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  max-height: 55vh;
  min-height: 0;
  border-top: 1px solid var(--color-border-default);
}

/* The header belongs to the fold, not the panel, so every panel gets the name and the minimize
   dash without drawing its own. */
.panel-head {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-1) var(--space-2) 0 var(--space-3);
}

.panel-title {
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
  font-weight: 600;
}

.panel-bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-top: 1px solid var(--color-border-default);
}

.sidebar-tree {
  flex-shrink: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--color-surface-1);
}

.sidebar-tree .view-switch {
  margin: var(--space-3) var(--space-2) 0;
}

.sidebar-gutter {
  flex-shrink: 0;
  width: 4px;
  cursor: col-resize;
  background: var(--color-border-subtle);
  transition: background var(--duration-fast) var(--ease-out);
}

.sidebar-gutter:hover,
.sidebar-gutter:active {
  background: var(--color-amber);
}
</style>
