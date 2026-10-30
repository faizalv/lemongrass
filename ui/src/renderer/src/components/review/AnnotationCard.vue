<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import AppButton from '../AppButton.vue'

// A draft has no saved comment yet, so cancelling it discards it instead of returning to view.
const props = defineProps<{
  comment: string
  draft: boolean
}>()

const emit = defineEmits<{
  save: [comment: string]
  cancel: []
  remove: []
}>()

const editing = ref(props.draft)
const text = ref(props.comment)
const inputRef = ref<HTMLTextAreaElement | null>(null)

async function startEditing(): Promise<void> {
  text.value = props.comment
  editing.value = true
  await nextTick()
  inputRef.value?.focus()
}

function save(): void {
  const comment = text.value.trim()
  if (!comment) return
  emit('save', comment)
  if (!props.draft) editing.value = false
}

function cancel(): void {
  if (props.draft) emit('cancel')
  else editing.value = false
}

onMounted(() => {
  if (editing.value) inputRef.value?.focus()
})
</script>

<template>
  <div class="annotation-card">
    <template v-if="editing">
      <textarea
        ref="inputRef"
        v-model="text"
        class="input"
        rows="3"
        placeholder="Leave a comment"
        @keydown.escape.stop.prevent="cancel"
        @keydown.enter.ctrl.exact.prevent="save"
        @keydown.enter.meta.exact.prevent="save"
      />
      <div class="actions">
        <AppButton type="button" @click="cancel">Cancel</AppButton>
        <AppButton type="button" variant="primary" :disabled="!text.trim()" @click="save">
          {{ draft ? 'Add comment' : 'Save' }}
        </AppButton>
      </div>
    </template>
    <template v-else>
      <p class="comment">{{ comment }}</p>
      <div class="actions">
        <AppButton type="button" variant="danger" @click="emit('remove')">Delete</AppButton>
        <AppButton type="button" @click="startEditing">Edit</AppButton>
      </div>
    </template>
  </div>
</template>

<style scoped>
.annotation-card {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: var(--space-1) var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-1);
  border: 1px solid var(--color-border-default);
  border-left: 3px solid var(--color-amber);
  border-radius: var(--radius-md);
  font-family: var(--font-body);
}

.input {
  box-sizing: border-box;
  width: 100%;
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-2);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  resize: vertical;
}

.input:focus {
  outline: none;
  border-color: var(--color-amber);
}

.comment {
  margin: 0;
  color: var(--color-fg-primary);
  font-size: var(--text-xs);
  white-space: pre-wrap;
  word-break: break-word;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}
</style>
