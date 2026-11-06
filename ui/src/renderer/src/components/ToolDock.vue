<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppButton, AppContextMenu, AppIconButton, AppPad, type MenuItem } from '@sankasten'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import NodeModal from './NodeModal.vue'
import TabIcon from './TabIcon.vue'
import { pinIcon, toolboxIcon } from '../icons'
import { closeNodeModal, nodeStatus, startNodeStatus } from '../nodeStatus'
import {
  isAlwaysPinned,
  isPinned,
  loadToolDock,
  pinnedTools,
  togglePin,
  toolDock
} from '../toolDock'
import { TOOL_KINDS, toolTitle, type ToolId, type ToolKind } from '../toolKinds'
import type { ProjectRef } from '../workspace'

const props = defineProps<{
  project: ProjectRef
}>()

const PIN_PATH = pinIcon[0].attrs.d as string

const rootRef = ref<HTMLElement | null>(null)
const menu = ref<{ x: number; y: number; id: ToolId } | null>(null)

const pads = computed(() => TOOL_KINDS.map((kind) => ({ kind, status: kind.status() })))

const MENU_ITEMS: MenuItem[] = [{ key: 'unpin', label: 'Unpin from the header', icon: [PIN_PATH] }]

// The tool's own open focuses its tab when one exists, so the dock never opens a second one.
function openTool(kind: ToolKind): void {
  toolDock.drawerOpen = false
  kind.open(props.project)
}

// An always-pinned tool has nothing to offer in the menu, so it opens none.
function openMenu(event: MouseEvent, id: ToolId): void {
  if (isAlwaysPinned(id)) return
  menu.value = { x: event.clientX, y: event.clientY, id }
}

function onMenuSelect(): void {
  if (menu.value) togglePin(menu.value.id)
  menu.value = null
}

function onDocumentClick(event: MouseEvent): void {
  if (toolDock.drawerOpen && rootRef.value && !rootRef.value.contains(event.target as Node)) {
    toolDock.drawerOpen = false
  }
}

function onKey(event: KeyboardEvent): void {
  if (toolDock.drawerOpen && event.key === 'Escape') toolDock.drawerOpen = false
}

onMounted(() => {
  void loadToolDock()
  startNodeStatus()
  document.addEventListener('click', onDocumentClick, true)
  document.addEventListener('keydown', onKey)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocumentClick, true)
  document.removeEventListener('keydown', onKey)
})
</script>

<template>
  <div ref="rootRef" class="tool-dock" role="toolbar" aria-label="Tools">
    <AppButton
      v-for="kind in pinnedTools()"
      :key="kind.id"
      variant="text"
      size="sm"
      :title="kind.label"
      :aria-label="kind.label"
      @click="openTool(kind)"
      @contextmenu.prevent="openMenu($event, kind.id)"
    >
      <TabIcon :shapes="kind.icon" />
      <span class="dock-label">{{ kind.label }}</span>
    </AppButton>

    <AppIconButton
      label="Tools"
      size="sm"
      :pressed="toolDock.drawerOpen"
      @click="toolDock.drawerOpen = !toolDock.drawerOpen"
    >
      <TabIcon :shapes="toolboxIcon" />
    </AppIconButton>

    <div v-if="toolDock.drawerOpen" class="drawer sk-float">
      <p class="drawer-title">Tools</p>
      <div class="pads">
        <AppPad
          v-for="{ kind, status } in pads"
          :key="kind.id"
          size="sm"
          :label="kind.label"
          :detail="status.text"
          :tone="status.tone"
          :title="toolTitle(kind, status)"
          @click="openTool(kind)"
        >
          <template #icon><TabIcon :shapes="kind.icon" :size="16" /></template>
          <template v-if="!isAlwaysPinned(kind.id)" #corner>
            <AppIconButton
              size="xs"
              :tone="isPinned(kind.id) ? 'accent' : 'neutral'"
              :pressed="isPinned(kind.id)"
              :label="isPinned(kind.id) ? 'Unpin from the header' : 'Pin to the header'"
              @click="togglePin(kind.id)"
            >
              <TabIcon :shapes="pinIcon" />
            </AppIconButton>
          </template>
        </AppPad>
      </div>
    </div>

    <AppContextMenu
      v-if="menu"
      :x="menu.x"
      :y="menu.y"
      :items="MENU_ITEMS"
      @select="onMenuSelect"
      @close="menu = null"
    />
    <NodeModal v-if="nodeStatus.modalOpen" @close="closeNodeModal" />
  </div>
</template>

<style scoped>
.tool-dock {
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--space-1);
  -webkit-app-region: no-drag;
}

/* A narrow header keeps only the glyphs, so the dock never reaches the title or the controls. */
@container header (max-width: 1440px) {
  .dock-label {
    display: none;
  }
}

.drawer {
  position: absolute;
  top: calc(100% + var(--space-2));
  left: 50%;
  z-index: 90;
  width: 480px;
  padding: var(--space-3);
  transform: translateX(-50%);
}

.drawer-title {
  margin: 0 0 var(--space-2);
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
}

.pads {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-2);
}
</style>
