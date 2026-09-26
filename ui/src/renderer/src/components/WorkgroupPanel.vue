<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { disbandWorkgroup, refreshWorkgroups, workgroupStateOf } from '../workgroups'
import { focusTab, isTabOpen, liveShellCount, type ProjectRef } from '../workspace'
import type { WorkgroupInfo, WorkgroupMemberInfo } from '../../../preload/types'

const POLL_MS = 5000

const props = defineProps<{
  project: ProjectRef
}>()

const state = computed(() => workgroupStateOf(props.project.id))
const groups = computed(() => state.value.groups)
const memberCount = computed(() =>
  groups.value.reduce((total, group) => total + group.members.length, 0)
)
const summary = computed(() => {
  if (groups.value.length === 0) return 'None active'
  return groups.value.length === 1 ? groups.value[0].name : `${groups.value.length} groups`
})

const ROLE_LETTERS: Record<WorkgroupMemberInfo['role'], string> = { pilot: 'P', copilot: 'C' }
const ROLE_LABELS: Record<WorkgroupMemberInfo['role'], string> = {
  pilot: 'Pilot',
  copilot: 'Co-pilot'
}

function online(member: WorkgroupMemberInfo): boolean {
  return isTabOpen(props.project.id, member.tabId)
}

function orphaned(group: WorkgroupInfo): boolean {
  return !isTabOpen(props.project.id, group.pilotTabId)
}

function memberTitle(member: WorkgroupMemberInfo): string {
  const status = online(member) ? 'open in this window' : 'tab closed'
  return `${ROLE_LABELS[member.role]} ${member.label} (${member.vendor}), ${status}`
}

function refresh(): void {
  void refreshWorkgroups(props.project)
}

function onFocus(): void {
  refresh()
}

let timer: ReturnType<typeof setInterval> | undefined

onMounted(() => {
  refresh()
  timer = setInterval(() => {
    if (document.hasFocus()) refresh()
  }, POLL_MS)
  window.addEventListener('focus', onFocus)
})

onBeforeUnmount(() => {
  clearInterval(timer)
  window.removeEventListener('focus', onFocus)
})

watch(() => props.project.id, refresh)
// A workgroup's tabs open right after the group is created, so a change in the shell count is the cue to look again.
watch(() => liveShellCount(props.project.id), refresh)
</script>

<template>
  <div class="workgroup-panel">
    <button
      class="wg-item"
      :class="{ open: !state.folded }"
      :aria-expanded="!state.folded"
      @click="state.folded = !state.folded"
    >
      <svg
        width="13"
        height="13"
        viewBox="0 0 14 14"
        fill="none"
        stroke="currentColor"
        stroke-width="1.2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <circle cx="7" cy="3.5" r="1.7" />
        <circle cx="3" cy="10.5" r="1.7" />
        <circle cx="11" cy="10.5" r="1.7" />
        <path d="M6 5l-2 4M8 5l2 4M4.7 10.5h4.6" />
      </svg>
      <span class="wg-label">Workgroup</span>
      <span class="wg-summary" :title="summary">{{ summary }}</span>
      <span v-if="groups.length > 0" class="wg-count">{{ memberCount }}</span>
      <svg
        class="wg-chevron"
        width="10"
        height="10"
        viewBox="0 0 10 10"
        fill="none"
        stroke="currentColor"
        stroke-width="1.2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M3 2l3 3-3 3" />
      </svg>
    </button>

    <div v-if="!state.folded" class="wg-body">
      <p v-if="groups.length === 0" class="wg-empty">
        No workgroup is active. A pilot agent can start co-pilots in other tabs of this project and
        coordinate them through one shared thread. Ask an agent to set one up.
      </p>
      <div v-for="group in groups" :key="group.id" class="wg-group">
        <div class="group-head">
          <p class="wg-heading" :title="group.name">{{ group.name }}</p>
          <span v-if="orphaned(group)" class="wg-orphaned" title="The pilot's tab is not open">
            Orphaned
          </span>
          <span class="wg-thread">Thread {{ group.threadId }}</span>
        </div>

        <ul class="member-list lg-scroll">
          <li v-for="member in group.members" :key="member.tabId" class="member-item">
            <button
              class="member-row"
              :disabled="!online(member)"
              :title="memberTitle(member)"
              @click="focusTab(project, member.tabId)"
            >
              <span class="member-role" :class="member.role">{{ ROLE_LETTERS[member.role] }}</span>
              <span class="member-name">{{ member.label }}</span>
              <span class="member-vendor">{{ member.vendor }}</span>
              <span class="member-state" :class="{ online: online(member) }">
                {{ online(member) ? 'Online' : 'Offline' }}
              </span>
            </button>
          </li>
        </ul>

        <p v-if="state.confirming === group.id" class="wg-hint">
          Disband this workgroup? Its tabs stay open and its thread stays readable.
        </p>
        <div class="wg-actions">
          <template v-if="state.confirming === group.id">
            <button class="ghost-button" :disabled="state.busy" @click="state.confirming = null">
              Cancel
            </button>
            <button
              class="primary-button"
              :disabled="state.busy"
              @click="disbandWorkgroup(project, group.id)"
            >
              Disband
            </button>
          </template>
          <button v-else class="ghost-button" @click="state.confirming = group.id">Disband</button>
        </div>
      </div>
      <p v-if="state.error" class="wg-error lg-scroll">{{ state.error }}</p>
    </div>
  </div>
</template>

<style scoped>
.workgroup-panel {
  flex-shrink: 0;
  padding: var(--space-2);
  border-top: 1px solid var(--color-border-default);
}

.wg-item {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-2);
  border: none;
  border-radius: var(--radius-md);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.wg-item:hover {
  background: var(--color-surface-3);
}

.wg-item.open {
  background: var(--color-amber-muted);
  color: var(--color-fg-accent);
}

.wg-label {
  flex-shrink: 0;
}

.wg-summary {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-secondary);
  font-weight: var(--weight-regular);
  text-align: left;
}

.wg-count {
  flex-shrink: 0;
  min-width: 18px;
  padding: 0 6px;
  border-radius: var(--radius-pill);
  background: var(--color-amber);
  color: var(--color-black);
  font-size: 11px;
  line-height: 18px;
  text-align: center;
}

.wg-chevron {
  flex-shrink: 0;
  margin-left: auto;
  transition: transform var(--duration-fast) var(--ease-out);
}

.wg-item.open .wg-chevron {
  transform: rotate(90deg);
}

.wg-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  max-height: 55vh;
  overflow-y: auto;
  padding: var(--space-2) var(--space-1) 0;
}

.wg-empty {
  padding: 0 var(--space-2) var(--space-1);
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.wg-group {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.group-head {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 var(--space-2);
}

.wg-heading {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
}

.wg-orphaned {
  flex-shrink: 0;
  color: var(--color-warning);
  font-size: var(--text-xs);
}

.wg-thread {
  flex-shrink: 0;
  margin-left: auto;
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.member-list {
  flex-shrink: 0;
  max-height: 180px;
  overflow-y: auto;
  list-style: none;
  margin: 0;
  padding: 0;
}

.member-item {
  display: flex;
  align-items: center;
  padding-left: var(--space-2);
}

.member-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
  padding: 3px var(--space-2);
  background: transparent;
  border: none;
  border-radius: var(--radius-sm);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  text-align: left;
  cursor: pointer;
}

.member-row:hover:not(:disabled) {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.member-row:disabled {
  color: var(--color-fg-muted);
  cursor: default;
}

.member-role {
  flex-shrink: 0;
  width: 12px;
  font-family: var(--font-mono);
  font-weight: var(--weight-bold);
  text-align: center;
}

.member-role.pilot {
  color: var(--color-fg-accent);
}

.member-name {
  flex-shrink: 0;
  max-width: 50%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.member-vendor {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-muted);
}

.member-state {
  flex-shrink: 0;
  color: var(--color-fg-muted);
}

.member-state.online {
  color: var(--color-success);
}

.wg-hint {
  padding: 0 var(--space-2);
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
}

.wg-error {
  max-height: 120px;
  overflow-y: auto;
  padding: var(--space-2);
  background: var(--color-error-muted);
  border-radius: var(--radius-md);
  color: var(--color-error);
  font-size: var(--text-xs);
  white-space: pre-wrap;
  word-break: break-word;
}

.wg-actions {
  display: flex;
  gap: var(--space-2);
  padding-bottom: var(--space-1);
}

.ghost-button,
.primary-button {
  flex: 1;
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-pill);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  white-space: nowrap;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.ghost-button {
  background: transparent;
  border: 1px solid var(--color-border-default);
  color: var(--color-fg-secondary);
}

.ghost-button:hover:not(:disabled) {
  color: var(--color-fg-primary);
  background: var(--color-surface-2);
}

.primary-button {
  background: var(--color-amber);
  border: none;
  color: var(--color-black);
}

.primary-button:hover:not(:disabled) {
  background: var(--color-amber-dim);
}

.ghost-button:disabled,
.primary-button:disabled {
  opacity: 0.5;
  cursor: default;
}
</style>
