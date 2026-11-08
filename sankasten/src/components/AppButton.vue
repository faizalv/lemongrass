<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { ref } from 'vue'

withDefaults(
  defineProps<{
    // text is frameless, for a secondary action in a toolbar or a row. tone colors text and
    // ghost; the other variants carry their own color. pressed marks a toggle that is on.
    variant?: 'primary' | 'ghost' | 'danger' | 'text'
    // xs only for an action inside a list, tree or menu row.
    size?: 'xs' | 'sm' | 'md'
    tone?: 'neutral' | 'accent' | 'info' | 'success' | 'danger'
    pressed?: boolean
  }>(),
  { variant: 'ghost', size: 'md', tone: 'neutral', pressed: undefined }
)

const el = ref<HTMLButtonElement | null>(null)

defineExpose({ focus: () => el.value?.focus() })
</script>

<!-- Attributes such as type, disabled and click handlers fall through to the native button, and
     with no type set it keeps the browser default, so a button inside a form still submits it. -->
<template>
  <button
    ref="el"
    class="app-button"
    :class="[variant, `app-button--${size}`, `tone-${tone}`, { 'is-pressed': pressed }]"
    :aria-pressed="pressed"
  >
    <slot />
  </button>
</template>

<style scoped>
/* The height comes from the size token and every variant has a 1px border, transparent where it
   is not drawn, so primary, ghost, danger and text buttons of one size are the same height. */
.app-button {
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-1);
  height: var(--control-h-md);
  padding: 0 var(--space-4);
  border: 1px solid transparent;
  border-radius: var(--radius-pill);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  white-space: nowrap;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.app-button--sm {
  height: var(--control-h-sm);
  padding: 0 var(--space-3);
}

.app-button--xs {
  height: var(--control-h-xs);
  padding: 0 var(--space-2);
}

.app-button:focus-visible {
  outline: 2px solid var(--color-fg-accent);
  outline-offset: 2px;
}

.app-button:disabled {
  opacity: 0.5;
  cursor: default;
}

.primary {
  background: var(--color-amber);
  color: var(--color-on-fill);
}

.primary:hover:not(:disabled) {
  background: var(--color-amber-dim);
}

/* A faded amber reads as olive next to live amber, so a disabled primary is neutral instead. */
.primary:disabled {
  opacity: 1;
  background: var(--color-surface-3);
  color: var(--color-fg-muted);
}

.ghost {
  background: transparent;
  border-color: var(--color-border-default);
  color: var(--tone, var(--color-fg-secondary));
}

.ghost:hover:not(:disabled),
.text:hover:not(:disabled) {
  color: var(--tone, var(--color-fg-primary));
  background: var(--color-surface-2);
}

.danger {
  background: transparent;
  border-color: var(--color-error);
  color: var(--color-error);
}

.danger:hover:not(:disabled) {
  background: var(--color-error-muted);
}

.text {
  background: transparent;
  color: var(--tone, var(--color-fg-secondary));
}

/* A tone colors the label of a ghost or text button and keeps it on hover. */
.tone-accent {
  --tone: var(--color-amber);
}

.tone-info {
  --tone: var(--color-info);
}

.tone-success {
  --tone: var(--color-success);
}

.tone-danger {
  --tone: var(--color-error);
}

/* Pressed has no hover look of its own: it stays amber, since the grey hover reads as off. */
.ghost.is-pressed,
.text.is-pressed,
.ghost.is-pressed:hover:not(:disabled),
.text.is-pressed:hover:not(:disabled) {
  background: var(--color-amber-muted);
  color: var(--color-amber);
}
</style>
