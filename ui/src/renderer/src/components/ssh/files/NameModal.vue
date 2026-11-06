<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// New folder, New file and Rename: one name, Cancel and Apply. The modal stays open with the
// server's reason when the action fails, such as a name that is taken.
import { computed, onMounted, ref } from 'vue'
import { AppButton, AppInput, AppModal, type AppInputHandle } from '@sankasten'
import { nameProblem } from './remotePath'

const props = defineProps<{
  title: string
  initial: string
  apply: (name: string) => Promise<string | null>
}>()

const emit = defineEmits<{ close: [] }>()

const name = ref(props.initial)
const error = ref('')
const busy = ref(false)
const input = ref<AppInputHandle | null>(null)
const formEl = ref<HTMLFormElement | null>(null)
const trimmed = computed(() => name.value.trim())
const problem = computed(() => (trimmed.value ? nameProblem(trimmed.value) : null))
const canApply = computed(
  () => !!trimmed.value && !problem.value && trimmed.value !== props.initial && !busy.value
)

onMounted(() => {
  input.value?.focus()
  // A rename selects the name up to its extension, so typing replaces the stem.
  const dot = props.initial.lastIndexOf('.')
  const field = formEl.value?.querySelector('input')
  if (field && props.initial) field.setSelectionRange(0, dot > 0 ? dot : props.initial.length)
})

async function submit(): Promise<void> {
  if (!canApply.value) return
  busy.value = true
  error.value = ''
  try {
    const failed = await props.apply(trimmed.value)
    if (failed) error.value = failed
    else emit('close')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AppModal :title="title" @close="emit('close')">
    <form ref="formEl" class="name-form" @submit.prevent="submit">
      <AppInput ref="input" v-model="name" aria-label="Name" :aria-invalid="!!problem" />
      <p v-if="problem || error" class="name-error" role="alert">{{ problem ?? error }}</p>
    </form>
    <template #footer>
      <AppButton variant="text" @click="emit('close')">Cancel</AppButton>
      <AppButton variant="primary" :disabled="!canApply" @click="submit">Apply</AppButton>
    </template>
  </AppModal>
</template>

<style scoped>
.name-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.name-error {
  margin: 0;
  color: var(--color-error);
  font-size: var(--text-xs);
}
</style>
