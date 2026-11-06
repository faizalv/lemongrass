<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
// A count or a short tag in a pill, such as waiting items on a nav item, a disclosure header or an
// icon button's badge slot. The caller decides when to show it; it renders whatever it is given.
withDefaults(
  defineProps<{
    tone?: 'accent' | 'info' | 'success' | 'danger' | 'neutral'
    size?: 'sm' | 'md'
  }>(),
  { tone: 'accent', size: 'md' }
)
</script>

<template>
  <span class="app-badge" :class="[`tone-${tone}`, `size-${size}`]"><slot /></span>
</template>

<style scoped>
.app-badge {
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  height: 18px;
  min-width: 18px;
  padding: 0 6px;
  border-radius: var(--radius-pill);
  font-family: var(--font-body);
  font-size: 11px;
  font-weight: var(--weight-regular);
  line-height: 1;
  white-space: nowrap;
}

.size-sm {
  height: 16px;
  min-width: 16px;
  padding: 0 5px;
  font-size: 10px;
  font-weight: var(--weight-bold);
}

.tone-accent {
  background: var(--color-amber);
  color: var(--color-black);
}

.tone-info {
  background: var(--color-info);
  color: var(--color-white);
}

.tone-success {
  background: var(--color-success);
  color: var(--color-black);
}

.tone-danger {
  background: var(--color-error);
  color: var(--color-black);
}

.tone-neutral {
  background: var(--color-surface-3);
  color: var(--color-fg-secondary);
}
</style>
