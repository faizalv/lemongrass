<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppIconButton } from '@sankasten'
import { onMounted } from 'vue'
import GitPanelButton from './GitPanelButton.vue'
import NodeModal from './NodeModal.vue'
import TabIcon from './TabIcon.vue'
import WorkgroupPanelButton from './WorkgroupPanelButton.vue'
import { selectSidebarPane, sidebarSettings, toggleSidebarPanel } from '../explorerSettings'
import { filesIcon, sshIcon } from '../icons'
import { closeNodeModal, nodeStatus, startNodeStatus } from '../nodeStatus'
import { TOOL_KINDS, toolDot, toolTitle } from '../toolKinds'
import type { ProjectRef } from '../workspace'

defineProps<{
  project: ProjectRef
}>()

function shown(pane: typeof sidebarSettings.pane): boolean {
  return !sidebarSettings.hidden && sidebarSettings.pane === pane
}

// The rail lives as long as a project is open, so it owns the app-wide node poll the Node
// button's dot reads.
onMounted(startNodeStatus)
</script>

<template>
  <nav class="sidebar-rail" aria-label="Sidebar">
    <div class="rail-group" role="toolbar" aria-label="Views">
      <AppIconButton
        label="Files"
        size="sm"
        :pressed="shown('files')"
        @click="selectSidebarPane('files')"
      >
        <TabIcon :shapes="filesIcon" />
      </AppIconButton>
      <WorkgroupPanelButton
        :project="project"
        :open="shown('workgroup')"
        @toggle="selectSidebarPane('workgroup')"
      />
      <AppIconButton
        label="SSH"
        size="sm"
        :pressed="shown('ssh')"
        @click="selectSidebarPane('ssh')"
      >
        <TabIcon :shapes="sshIcon" />
      </AppIconButton>
    </div>

    <div class="rail-group" role="toolbar" aria-label="Tools">
      <GitPanelButton
        :project="project"
        :open="!sidebarSettings.hidden && sidebarSettings.panel === 'git'"
        @toggle="toggleSidebarPanel('git')"
      />
      <AppIconButton
        v-for="kind in TOOL_KINDS"
        :key="kind.id"
        size="sm"
        :label="toolTitle(kind, kind.status())"
        :dot="toolDot(kind.status())"
        @click="kind.open(project)"
      >
        <TabIcon :shapes="kind.icon" />
      </AppIconButton>
    </div>

    <NodeModal v-if="nodeStatus.modalOpen" @close="closeNodeModal" />
  </nav>
</template>

<style scoped>
/* Stays when the column is hidden, so every view and tool is one click away; scrolls rather than
   clipping when the window is too short for both groups. */
.sidebar-rail {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-1);
  overflow-y: auto;
  background: var(--color-surface-1);
  border-right: 1px solid var(--color-border-default);
}

.rail-group {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-1);
}
</style>
