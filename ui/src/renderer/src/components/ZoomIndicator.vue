<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { zoom, zoomLabel } from '../zoom'

const SHOWN_MS = 1200

const shown = ref(false)
let hideTimer: ReturnType<typeof setTimeout> | undefined

watch(
  () => zoom.changes,
  () => {
    shown.value = true
    clearTimeout(hideTimer)
    hideTimer = setTimeout(() => (shown.value = false), SHOWN_MS)
  }
)

onBeforeUnmount(() => clearTimeout(hideTimer))
</script>

<template>
  <Transition name="zoom-indicator">
    <div v-if="shown" class="zoom-indicator" role="status">Zoom {{ zoomLabel(zoom.factor) }}</div>
  </Transition>
</template>

<style scoped>
.zoom-indicator {
  position: fixed;
  top: var(--space-8);
  left: 50%;
  z-index: 900;
  transform: translateX(-50%);
  padding: var(--space-2) var(--space-4);
  background: var(--color-surface-2);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-lg);
  color: var(--color-fg-primary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  font-variant-numeric: tabular-nums;
  pointer-events: none;
}

.zoom-indicator-enter-active,
.zoom-indicator-leave-active {
  transition: opacity 0.15s ease;
}

.zoom-indicator-enter-from,
.zoom-indicator-leave-to {
  opacity: 0;
}
</style>
