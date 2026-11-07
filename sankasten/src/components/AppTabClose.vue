<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import AppConfirmButton from './AppConfirmButton.vue'

// A tab's close mark. Clean, it is a bare x. Dirty, it is a dot that turns into the x on hover or
// focus and confirms in place, since closing would drop unsaved work. The click never reaches
// the tab, so closing does not also select it.
withDefaults(
  defineProps<{
    // The close action's name, such as "Close notes.txt"; title and aria-label.
    label: string
    dirty?: boolean
    // Title and aria-label while a dirty close is armed.
    confirmLabel?: string
    size?: 'sm' | 'md'
  }>(),
  { dirty: false, confirmLabel: 'Close without saving', size: 'md' }
)

const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <AppConfirmButton
    v-if="dirty"
    v-slot="{ armed }"
    class="app-tab-close dirty"
    :class="`size-${size}`"
    :title="`${label}, unsaved changes`"
    :aria-label="`${label}, unsaved changes`"
    :confirm-label="confirmLabel"
    @click.stop
    @confirm="emit('close')"
  >
    <span class="dot" aria-hidden="true" />
    <span class="mark" aria-hidden="true">{{ armed ? '!' : '×' }}</span>
  </AppConfirmButton>
  <button
    v-else
    type="button"
    class="app-tab-close"
    :class="`size-${size}`"
    :title="label"
    :aria-label="label"
    @click.stop="emit('close')"
  >
    <span class="mark" aria-hidden="true">×</span>
  </button>
</template>

<style scoped>
.app-tab-close {
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  padding: 0;
  border: none;
  border-radius: var(--radius-pill);
  background: none;
  color: var(--color-fg-muted);
  font-family: var(--font-body);
  line-height: 1;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.size-sm {
  width: 18px;
  height: 18px;
  font-size: var(--text-md);
}

.size-md {
  width: 22px;
  height: 22px;
  font-size: var(--text-lg);
}

.app-tab-close:hover {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.app-tab-close:focus-visible {
  outline: 1px solid var(--color-amber);
  outline-offset: -1px;
}

.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--color-fg-secondary);
}

.dirty .mark {
  display: none;
}

.dirty:hover .dot,
.dirty:focus-visible .dot,
.dirty:not([data-confirm='idle']) .dot {
  display: none;
}

.dirty:hover .mark,
.dirty:focus-visible .mark,
.dirty:not([data-confirm='idle']) .mark {
  display: inline;
}
</style>
