<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppIcon } from '@sankasten'

// What the agent is doing with something the human is waiting on: a spinner while its tab's turn
// runs, a plain line when it is idle, so the human never stares at a still screen guessing.
defineProps<{
  working: boolean
  workingText: string
  idleText: string
}>()
</script>

<template>
  <p class="agent-working" :class="{ working }" role="status">
    <AppIcon v-if="working" class="spinner" name="spinner" :size="12" />
    <span>{{ working ? workingText : idleText }}</span>
  </p>
</template>

<style scoped>
.agent-working {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-fg-muted);
}

.agent-working.working {
  color: var(--color-fg-secondary);
}

.spinner {
  flex-shrink: 0;
  color: var(--color-violet);
  animation: spin 0.9s linear infinite;
}
</style>
