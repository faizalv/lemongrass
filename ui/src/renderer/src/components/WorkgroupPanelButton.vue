<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppIconButton } from '@sankasten'
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { pendingFor, refreshWorkgroups, workgroupStateOf } from '../workgroups'
import { liveShellCount, type ProjectRef } from '../workspace'

const POLL_MS = 5000

const props = defineProps<{
  project: ProjectRef
  open: boolean
}>()

defineEmits<{ toggle: [] }>()

const groups = computed(() => workgroupStateOf(props.project.id).groups.length)
const waiting = computed(() => pendingFor(props.project.path).length)

// A status dot, not a count, so the glyph stays readable: red while a group waits for approval,
// amber while any group is live. The label carries the numbers.
const dot = computed(() => (waiting.value > 0 ? 'danger' : groups.value > 0 ? 'accent' : undefined))

const label = computed(() => {
  const parts = ['Workgroup']
  if (groups.value > 0) parts.push(`${groups.value} active`)
  if (waiting.value > 0) parts.push(`${waiting.value} waiting for your approval`)
  return parts.join(', ')
})

function refresh(): void {
  void refreshWorkgroups(props.project)
}

let timer: ReturnType<typeof setInterval> | undefined

// The button owns the poll, not the panel, so the count stays live while the panel is closed.
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
// A workgroup's tabs open right after the group is created, so a change in the shell count is the cue to look again.
watch(() => liveShellCount(props.project.id), refresh)
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
      <circle cx="7" cy="3.5" r="1.7" />
      <circle cx="3" cy="10.5" r="1.7" />
      <circle cx="11" cy="10.5" r="1.7" />
      <path d="M6 5l-2 4M8 5l2 4M4.7 10.5h4.6" />
    </svg>
  </AppIconButton>
</template>
