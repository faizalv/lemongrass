<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppActivity, AppBadge, AppEmptyState } from '@sankasten'
import { computed } from 'vue'
import { tabActivity } from '../tabStates'
import { openApprovals, pendingFor, workgroupStateOf } from '../workgroups'
import { focusedWorkgroupId, isTabOpen, openWorkgroup, type ProjectRef } from '../workspace'
import type { WorkgroupInfo } from '../../../preload/types'

const props = defineProps<{
  project: ProjectRef
}>()

const state = computed(() => workgroupStateOf(props.project.id))
const groups = computed(() => state.value.groups)
const waiting = computed(() => pendingFor(props.project.path))
const focusedId = computed(() => focusedWorkgroupId(props.project.id))

function orphaned(group: WorkgroupInfo): boolean {
  return !isTabOpen(props.project.id, group.leaderTabId)
}

// A member held up on a prompt outranks the ones working, since only the human can move it on.
function groupActivity(
  group: WorkgroupInfo
): { state: 'working' | 'waiting'; count: number } | null {
  const open = group.members.filter((member) => isTabOpen(props.project.id, member.tabId))
  const waiting = open.filter((member) => tabActivity(member.tabId) === 'waiting').length
  if (waiting > 0) return { state: 'waiting', count: waiting }
  const working = open.filter((member) => tabActivity(member.tabId) === 'working').length
  return working > 0 ? { state: 'working', count: working } : null
}
</script>

<template>
  <div class="workgroup-panel">
    <div class="wg-body">
      <button v-if="waiting.length > 0" class="wg-row waiting" @click="openApprovals()">
        <span class="wg-row-name">Waiting approval</span>
        <AppBadge>{{ waiting.length }}</AppBadge>
      </button>
      <button
        v-for="group in groups"
        :key="group.id"
        class="wg-row"
        :class="{ shown: focusedId === group.id }"
        :title="group.name"
        @click="openWorkgroup(project, group.id)"
      >
        <span class="wg-row-name">{{ group.name }}</span>
        <span
          v-if="groupActivity(group)"
          class="wg-activity"
          :title="
            groupActivity(group)!.state === 'waiting'
              ? `${groupActivity(group)!.count} waiting for you`
              : `${groupActivity(group)!.count} working`
          "
        >
          <AppActivity :state="groupActivity(group)!.state" :size="11" />
          {{ groupActivity(group)!.count }}
        </span>
        <span v-if="orphaned(group)" class="wg-orphaned" title="The leader's tab is not open">
          Orphaned
        </span>
      </button>
      <AppEmptyState
        v-if="groups.length === 0 && waiting.length === 0"
        size="sm"
        title="No workgroup is active."
        detail="A leader agent can start thinkers in other tabs of this project and coordinate them through one shared thread. Ask an agent to set one up."
      />
    </div>
  </div>
</template>

<style scoped>
.workgroup-panel {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.wg-body {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow-y: auto;
  padding: var(--space-2) var(--space-3);
}

.wg-row {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: var(--space-1) var(--space-3);
  background: var(--layer-raised);
  border: none;
  border-radius: var(--radius-sm);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  text-align: left;
  cursor: pointer;
}

.wg-row:hover {
  background: var(--layer-raised-hover);
  color: var(--color-fg-primary);
}

.wg-row.shown {
  background: var(--color-amber-muted);
  color: var(--color-fg-accent);
}

.wg-row.waiting {
  color: var(--color-fg-accent);
  font-weight: var(--weight-semibold);
}

.wg-row-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.wg-activity {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
  font-variant-numeric: tabular-nums;
}

.wg-orphaned {
  flex-shrink: 0;
  color: var(--color-warning);
  font-size: var(--text-xs);
}
</style>
