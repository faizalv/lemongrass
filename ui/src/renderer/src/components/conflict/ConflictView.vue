<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, onMounted, shallowRef, watch } from 'vue'
import { gitStateOf } from '../../git'
import { dropDraft } from '../../conflictDrafts'
import { conflictScope } from '../../conflictAnnotations'
import type { ProjectRef } from '../../workspace'
import type { GitConflictFile } from '../../../../preload/types'
import ConflictMerge from './ConflictMerge.vue'
import ConflictSides from './ConflictSides.vue'
import ReviewBar from '../review/ReviewBar.vue'

const props = defineProps<{
  project: ProjectRef
  path: string
}>()

const file = shallowRef<GitConflictFile | null>(null)
const okFile = computed(() => (file.value?.status === 'ok' ? file.value : null))
let loading = false

// Reloaded on every status refresh, so a file the model changed or resolved shows here. The
// object is replaced only when it differs, since the merge screen reacts to every new one.
async function load(): Promise<void> {
  if (loading) return
  loading = true
  try {
    const next = await window.api.git.conflict(props.project.path, props.path)
    if (JSON.stringify(next) !== JSON.stringify(file.value)) file.value = next
    // A resolved file has no work left to keep for its tab.
    if (next.status === 'notConflicted') dropDraft(props.project.id, props.path)
  } catch (err) {
    file.value = { status: 'error', error: err instanceof Error ? err.message : String(err) }
  } finally {
    loading = false
  }
}

onMounted(load)
watch(() => gitStateOf(props.project.id).revision, load)
</script>

<template>
  <div class="conflict-view">
    <div v-if="!file" class="notice">Loading {{ path }}</div>
    <div v-else-if="file.status === 'notConflicted'" class="notice">
      {{ path }} is not in conflict. You can close this tab.
    </div>
    <div v-else-if="file.status === 'error'" class="notice error">{{ file.error }}</div>
    <template v-else-if="okFile">
      <ConflictMerge v-if="okFile.chunks" :project="project" :path="path" :file="okFile" />
      <ConflictSides v-else :project="project" :path="path" :file="okFile" />
      <ReviewBar
        :project="project"
        :scope="conflictScope(okFile.operation, okFile.leftRef, okFile.rightRef)"
      />
    </template>
  </div>
</template>

<style scoped>
.conflict-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--color-surface-0);
}

.notice {
  padding: var(--space-6);
  font-size: var(--text-sm);
  color: var(--color-fg-secondary);
}

.notice.error {
  color: var(--color-error);
}
</style>
