<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { ref, useId } from 'vue'
import { AppDisclosure, AppIconButton, AppInfoPopover } from '@sankasten'

// The head of one login setup inside Authentication: what it signs in to, whether it is on, a
// one-line purpose, and an info icon whose popover says how to use it.
defineProps<{
  label: string
  purpose: string
  on: boolean
  open: boolean
  summary?: string
}>()

const emit = defineEmits<{ 'update:open': [open: boolean] }>()

const infoId = useId()
const anchor = ref<HTMLElement | null>(null)

function show(event: Event): void {
  anchor.value = event.currentTarget as HTMLElement
}

function hide(): void {
  anchor.value = null
}
</script>

<template>
  <div class="auth-side-head">
    <div class="settings-toggle">
      <AppDisclosure :open="open" :label="label" @update:open="emit('update:open', $event)" />
      <span class="status-capsule" :class="{ 'status-open': on }">{{ on ? 'On' : 'Off' }}</span>
      <span v-if="summary && !open" class="settings-summary">{{ summary }}</span>
      <span class="auth-side-fill"></span>
      <AppIconButton
        icon="info"
        size="sm"
        :label="`About ${label}`"
        :aria-describedby="anchor ? infoId : undefined"
        @mouseenter="show"
        @mouseleave="hide"
        @focus="show"
        @blur="hide"
      />
    </div>
    <p class="hint auth-side-purpose">{{ purpose }}</p>
    <AppInfoPopover :id="infoId" :anchor="anchor"><slot /></AppInfoPopover>
  </div>
</template>

<style scoped>
.auth-side-head {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  min-width: 0;
}

.auth-side-fill {
  flex: 1;
}

/* The disclosure's own bottom margin would lift it above the row's center line. */
.auth-side-head :deep(.app-disclosure) {
  margin-bottom: 0;
}

.auth-side-purpose {
  padding-left: 15px;
}
</style>
