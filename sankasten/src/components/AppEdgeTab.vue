<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
// A tab sticking out from the edge of a panel, such as a minimized view parked on a side panel:
// an icon, an optional count and a dot for something pending. It can lie over text, so it has a
// solid raised fill. The caller places it against the edge with its own layout class; the side
// it sticks out toward is rounded, the other is flat and open where it meets the panel.
withDefaults(
  defineProps<{
    // What a click does, such as "Show editor, 2 files open"; title and aria-label.
    label: string
    side?: 'left' | 'right'
    count?: number
    // An amber dot, for unsaved or pending work.
    dot?: boolean
  }>(),
  { side: 'left', count: undefined, dot: false }
)
</script>

<template>
  <button
    type="button"
    class="app-edge-tab"
    :class="`side-${side}`"
    :title="label"
    :aria-label="label"
  >
    <span v-if="$slots.icon" class="icon"><slot name="icon" /></span>
    <span v-if="count !== undefined" class="count">{{ count }}</span>
    <span v-if="dot" class="dot" aria-hidden="true" />
  </button>
</template>

<style scoped>
.app-edge-tab {
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  height: var(--control-h-sm);
  padding: 0 var(--space-2);
  border: 1px solid var(--color-border-default);
  background: var(--color-surface-2);
  box-shadow: var(--shadow-md);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-variant-numeric: tabular-nums;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.side-left {
  padding-left: var(--space-3);
  border-right: none;
  border-radius: var(--radius-md) 0 0 var(--radius-md);
}

.side-right {
  padding-right: var(--space-3);
  border-left: none;
  border-radius: 0 var(--radius-md) var(--radius-md) 0;
}

.app-edge-tab:hover {
  background: var(--color-surface-3);
  color: var(--color-fg-primary);
}

.app-edge-tab:focus-visible {
  outline: 1px solid var(--color-amber);
  outline-offset: -1px;
}

.icon {
  display: inline-flex;
  flex-shrink: 0;
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-amber);
}
</style>
