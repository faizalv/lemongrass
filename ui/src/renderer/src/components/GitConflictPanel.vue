<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script lang="ts">
import { reactive } from 'vue'

// Project ids whose Conflicted files drawer is open. It starts folded and stays as the human left
// it while the panel folds and unfolds, which remounts this component.
const openDrawers = reactive(new Set<string>())
</script>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { finishOperation, gitStateOf } from '../git'
import { openConflict, type ProjectRef } from '../workspace'
import type { GitFile } from '../../../preload/types'
import AppButton from './AppButton.vue'

const props = defineProps<{
  project: ProjectRef
}>()

const state = computed(() => gitStateOf(props.project.id))
const status = computed(() => state.value.status)
const error = ref<string | null>(null)
const conflicted = computed(() =>
  (status.value?.files ?? []).filter((file) => file.group === 'conflicted')
)
const drawerOpen = computed(() => openDrawers.has(props.project.id))

// The refs are split out so the sentence can set them apart; without refs it is a plain sentence.
const message = computed(() => {
  const refs = status.value?.operationRefs ?? null
  if (status.value?.operation === 'merge') {
    return refs
      ? { lead: 'Conflict while trying to merge', from: refs.from, join: 'into', onto: refs.onto }
      : { lead: 'Conflict while merging.' }
  }
  return refs
    ? { lead: 'Conflict while rebasing', from: refs.from, join: 'onto', onto: refs.onto }
    : { lead: 'Conflict while rebasing.' }
})

function fileName(file: GitFile): string {
  return file.path.slice(file.path.lastIndexOf('/') + 1)
}

function fileDir(file: GitFile): string {
  const index = file.path.lastIndexOf('/')
  return index === -1 ? '' : file.path.slice(0, index)
}

function blocksTitle(file: GitFile): string {
  if (file.conflictBlocks == null) return file.path
  const blocks =
    file.conflictBlocks === 1 ? '1 conflict block' : `${file.conflictBlocks} conflict blocks`
  return `${file.path}: ${blocks} left`
}

function toggleDrawer(): void {
  if (openDrawers.has(props.project.id)) openDrawers.delete(props.project.id)
  else openDrawers.add(props.project.id)
}

function resolve(): void {
  if (conflicted.value.length > 0) openConflict(props.project, conflicted.value[0].path)
}

async function finish(action: 'continue' | 'abort'): Promise<void> {
  error.value = await finishOperation(props.project, action)
}
</script>

<template>
  <div class="conflict-panel">
    <p class="conflict-message">
      {{ message.lead }}
      <template v-if="message.from">
        <span class="conflict-ref">{{ message.from }}</span>
        {{ message.join }}
        <span class="conflict-ref">{{ message.onto }}</span>
      </template>
    </p>
    <p v-if="error" class="conflict-error">{{ error }}</p>
    <div class="conflict-buttons">
      <AppButton
        v-if="conflicted.length > 0"
        variant="primary"
        class="conflict-action"
        :disabled="state.busy !== null"
        @click="resolve"
      >
        Resolve
      </AppButton>
      <AppButton
        v-else
        variant="primary"
        class="conflict-action"
        :disabled="state.busy !== null"
        @click="finish('continue')"
      >
        Continue
      </AppButton>
      <AppButton
        variant="danger"
        class="conflict-action"
        :disabled="state.busy !== null"
        @click="finish('abort')"
      >
        Abort
      </AppButton>
    </div>

    <p v-if="conflicted.length === 0" class="conflict-empty">All conflicts are resolved.</p>
    <section v-else>
      <button class="drawer-head" :aria-expanded="drawerOpen" @click="toggleDrawer">
        <svg
          class="drawer-chevron"
          :class="{ open: drawerOpen }"
          width="9"
          height="9"
          viewBox="0 0 10 10"
          fill="none"
          stroke="currentColor"
          stroke-width="1.2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M3 2l3 3-3 3" />
        </svg>
        <span>Conflicted files</span>
        <span class="drawer-count">{{ conflicted.length }}</span>
      </button>
      <ul v-if="drawerOpen" class="conflict-files">
        <li v-for="file in conflicted" :key="file.path">
          <button
            class="conflict-file"
            :title="blocksTitle(file)"
            @click="openConflict(project, file.path)"
          >
            <span class="file-name">{{ fileName(file) }}</span>
            <span class="file-dir">{{ fileDir(file) }}</span>
            <span v-if="file.conflictBlocks != null" class="file-blocks">{{
              file.conflictBlocks
            }}</span>
          </button>
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.conflict-panel {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  font-size: var(--text-xs);
}

.conflict-message {
  padding: 0 var(--space-2);
  color: var(--color-fg-primary);
}

.conflict-ref {
  color: var(--color-fg-accent);
  font-family: var(--font-mono);
  font-weight: var(--weight-medium);
  word-break: break-all;
}

.conflict-error {
  max-height: 120px;
  overflow-y: auto;
  padding: 0 var(--space-2);
  color: var(--color-error);
  white-space: pre-wrap;
  word-break: break-word;
}

.conflict-buttons {
  display: flex;
  gap: var(--space-2);
  padding: 0 var(--space-2);
}

.conflict-buttons .conflict-action {
  flex: 1;
  padding: var(--space-1) var(--space-2);
}

.conflict-empty {
  padding: 0 var(--space-2);
  color: var(--color-fg-muted);
}

.drawer-head {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 3px var(--space-2);
  background: transparent;
  border: none;
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
  cursor: pointer;
}

.drawer-head:hover {
  color: var(--color-fg-primary);
}

.drawer-chevron {
  flex-shrink: 0;
  transition: transform var(--duration-fast) var(--ease-out);
}

.drawer-chevron.open {
  transform: rotate(90deg);
}

.drawer-count {
  color: var(--color-fg-muted);
  font-weight: var(--weight-regular);
}

.conflict-files {
  max-height: 240px;
  overflow-y: auto;
  list-style: none;
  margin: 0;
  padding: 0;
}

.conflict-file {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 3px var(--space-2) 3px var(--space-5);
  background: transparent;
  border: none;
  border-radius: var(--radius-sm);
  color: var(--color-warning);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  text-align: left;
  cursor: pointer;
}

.conflict-file:hover {
  background: var(--color-surface-2);
}

.file-name {
  flex-shrink: 0;
  max-width: 60%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-dir {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-fg-muted);
}

.file-blocks {
  flex-shrink: 0;
  color: var(--color-fg-muted);
  font-variant-numeric: tabular-nums;
}
</style>
