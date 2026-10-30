<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref } from 'vue'
import { finishOperation, gitStateOf } from '../git'
import { openConflict, type ProjectRef } from '../workspace'
import AppButton from './AppButton.vue'

const props = defineProps<{
  project: ProjectRef
}>()

const state = computed(() => gitStateOf(props.project.id))
const status = computed(() => state.value.status)
const error = ref<string | null>(null)
const firstConflict = computed(
  () => (status.value?.files ?? []).find((file) => file.group === 'conflicted') ?? null
)

const message = computed(() => {
  const refs = status.value?.operationRefs
  if (status.value?.operation === 'merge') {
    return refs
      ? `Conflict while trying to merge ${refs.from} into ${refs.onto}.`
      : 'Conflict while merging.'
  }
  return refs
    ? `Conflict while rebasing ${refs.from} onto ${refs.onto}.`
    : 'Conflict while rebasing.'
})

function resolve(): void {
  if (firstConflict.value) openConflict(props.project, firstConflict.value.path)
}

async function finish(action: 'continue' | 'abort'): Promise<void> {
  error.value = await finishOperation(props.project, action)
}
</script>

<template>
  <div v-if="status?.operation" class="operation-box">
    <p>{{ message }}</p>
    <p v-if="error" class="operation-error">{{ error }}</p>
    <div class="operation-buttons">
      <AppButton
        v-if="firstConflict"
        variant="primary"
        class="operation-action"
        :disabled="state.busy !== null"
        @click="resolve"
      >
        Resolve
      </AppButton>
      <AppButton
        v-else
        variant="primary"
        class="operation-action"
        :disabled="state.busy !== null"
        @click="finish('continue')"
      >
        Continue
      </AppButton>
      <AppButton
        variant="danger"
        class="operation-action"
        :disabled="state.busy !== null"
        @click="finish('abort')"
      >
        Abort
      </AppButton>
    </div>
  </div>
</template>

<style scoped>
.operation-box {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0 var(--space-2);
  padding: var(--space-2);
  background: var(--color-surface-2);
  border-radius: var(--radius-md);
  color: var(--color-fg-primary);
  font-size: var(--text-xs);
}

.operation-error {
  max-height: 120px;
  overflow-y: auto;
  color: var(--color-error);
  white-space: pre-wrap;
  word-break: break-word;
}

.operation-buttons {
  display: flex;
  gap: var(--space-2);
}

.operation-buttons .operation-action {
  flex: 1;
  padding: var(--space-1) var(--space-2);
}
</style>
