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

// The refs are split out so the line can show them as chips; without refs it is a plain sentence.
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

function resolve(): void {
  if (firstConflict.value) openConflict(props.project, firstConflict.value.path)
}

async function finish(action: 'continue' | 'abort'): Promise<void> {
  error.value = await finishOperation(props.project, action)
}
</script>

<template>
  <div v-if="status?.operation" class="operation-box">
    <p class="operation-message">
      {{ message.lead }}
      <template v-if="message.from">
        <span class="operation-ref">{{ message.from }}</span>
        {{ message.join }}
        <span class="operation-ref">{{ message.onto }}</span>
      </template>
    </p>
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

.operation-message {
  line-height: 1.8;
}

.operation-ref {
  padding: 1px var(--space-2);
  background: var(--color-surface-3);
  border-radius: var(--radius-sm);
  color: var(--color-fg-accent);
  font-family: var(--font-mono);
  font-weight: var(--weight-medium);
  word-break: break-all;
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
