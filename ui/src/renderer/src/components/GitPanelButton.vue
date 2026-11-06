<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppIconButton } from '@sankasten'
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { gitStateOf, refreshGit } from '../git'
import type { ProjectRef } from '../workspace'

const POLL_MS = 5000

const props = defineProps<{
  project: ProjectRef
  open: boolean
}>()

defineEmits<{ toggle: [] }>()

const status = computed(() => gitStateOf(props.project.id).status)
const changes = computed(() => status.value?.files.length ?? 0)
const unpushed = computed(() => (status.value?.isRepo ? status.value.ahead : 0))
const conflictMode = computed(() => !!status.value?.operation)
// A status dot, not a count, so the glyph stays readable: red for a stopped merge or rebase,
// amber for uncommitted changes. The label carries the numbers.
const dot = computed(() =>
  conflictMode.value ? 'danger' : changes.value > 0 ? 'accent' : undefined
)

const label = computed(() => {
  const parts = [status.value?.branch ? `Git: ${status.value.branch}` : 'Git']
  if (conflictMode.value) parts.push(`${status.value?.operation} stopped`)
  if (changes.value > 0) parts.push(`${changes.value} uncommitted`)
  if (unpushed.value > 0) parts.push(`${unpushed.value} unpushed`)
  return parts.join(', ')
})

function refresh(): void {
  void refreshGit(props.project)
}

let timer: ReturnType<typeof setInterval> | undefined

// The button owns the poll, not the panel, so the counts stay live while the panel is closed.
onMounted(() => {
  refresh()
  timer = setInterval(() => {
    if (document.hasFocus()) refresh()
  }, POLL_MS)
  window.addEventListener('focus', refresh)
})

onBeforeUnmount(() => {
  clearInterval(timer)
  window.removeEventListener('focus', refresh)
})

watch(() => props.project.id, refresh)
</script>

<template>
  <AppIconButton :label="label" size="sm" :dot="dot" :pressed="open" @click="$emit('toggle')">
    <svg
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      stroke-width="1.2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <circle cx="4" cy="3" r="1.5" />
      <circle cx="4" cy="11" r="1.5" />
      <circle cx="10.5" cy="5.5" r="1.5" />
      <path d="M4 4.5v5M10.5 7c0 2.2-2.2 2.9-6 3.2" />
    </svg>
    <template v-if="unpushed > 0" #trailing>
      <svg
        class="ahead"
        viewBox="0 0 10 10"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M5 8.5v-7M2 4.5l3-3 3 3" />
      </svg>
    </template>
  </AppIconButton>
</template>

<style scoped>
/* Green like the ahead count in the branch row. */
.ahead {
  color: var(--color-success);
}
</style>
