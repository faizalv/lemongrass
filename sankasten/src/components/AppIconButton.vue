<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { ref } from 'vue'
import AppConfirmButton from './AppConfirmButton.vue'
import AppIcon from './AppIcon.vue'
import type { AppIconName } from '../icons/appIcons'

// A square button holding only a glyph, for navigation, toggles and row actions. The label is
// required because it is the button's only name: it becomes the title and the aria-label. A
// caller's own title attribute still wins as the tooltip, for a disabled reason.
const props = withDefaults(
  defineProps<{
    // An app-wide glyph; without it the default slot draws the glyph (any icon component or a
    // short text such as B or H1).
    icon?: AppIconName
    label: string
    size?: 'sm' | 'md'
    // bare is the default for toolbars and rows; ghost adds a border; tinted fills in the tone.
    variant?: 'bare' | 'ghost' | 'tinted'
    // On bare it colors the glyph on hover only, on ghost and tinted it colors it always.
    tone?: 'neutral' | 'accent' | 'info' | 'success' | 'danger'
    // A toggle that is on, set as aria-pressed. Leave undefined for a plain action.
    pressed?: boolean
    // Work started by this button is running: the glyph becomes a spinner and clicks are ignored.
    busy?: boolean
    // A destructive action: the first click arms it and the second emits confirm instead of click.
    confirm?: boolean
    confirmLabel?: string
  }>(),
  {
    icon: undefined,
    size: 'md',
    variant: 'bare',
    tone: 'neutral',
    pressed: undefined,
    busy: false,
    confirm: false,
    confirmLabel: 'Click again'
  }
)

const emit = defineEmits<{ click: [event: MouseEvent]; confirm: [] }>()

const el = ref<HTMLButtonElement | { $el: HTMLButtonElement } | null>(null)

// click is declared, so a caller's @click arrives here and busy can hold it back.
function onClick(event: MouseEvent): void {
  if (props.busy || props.confirm) return
  emit('click', event)
}

function onConfirm(): void {
  if (!props.busy) emit('confirm')
}

defineExpose({
  focus: () => {
    const target = el.value && '$el' in el.value ? el.value.$el : el.value
    target?.focus()
  }
})
</script>

<!-- disabled and other attributes fall through to the native button. -->
<template>
  <component
    :is="confirm ? AppConfirmButton : 'button'"
    ref="el"
    type="button"
    class="app-icon-button"
    :class="[
      `variant-${variant}`,
      `size-${size}`,
      `tone-${tone}`,
      { 'is-pressed': pressed, 'is-busy': busy }
    ]"
    :title="label"
    :aria-label="label"
    :aria-pressed="pressed"
    :aria-busy="busy || undefined"
    v-bind="confirm ? { confirmLabel } : {}"
    @click="onClick"
    @confirm="onConfirm"
  >
    <AppIcon v-if="busy" class="busy-glyph" name="spinner" />
    <AppIcon v-else-if="icon" :name="icon" />
    <slot v-else />
    <span v-if="$slots.badge" class="badge"><slot name="badge" /></span>
  </component>
</template>

<style scoped>
/* Square at the control height. A transparent border on every variant keeps bare, ghost and
   tinted the same size, and min-width lets a short text glyph widen it instead of clipping. */
.app-icon-button {
  --h: var(--control-h-md);
  --glyph: var(--control-icon-md);
  --tone: var(--color-fg-primary);
  --tone-fill: var(--color-surface-3);
  position: relative;
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  height: var(--h);
  min-width: var(--h);
  padding: 0 var(--space-1);
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-fg-muted);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  font-weight: var(--weight-semibold);
  line-height: 1;
  white-space: nowrap;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
}

.size-sm {
  --h: var(--control-h-sm);
  --glyph: var(--control-icon-sm);
  font-size: var(--text-xs);
}

/* Any glyph, from the icon prop or the slot, is drawn at the size's glyph height; the width
   follows the glyph's own viewBox. */
.app-icon-button :deep(svg) {
  height: var(--glyph);
  width: auto;
}

.tone-accent {
  --tone: var(--color-amber);
  --tone-fill: var(--color-amber-muted);
}

.tone-info {
  --tone: var(--color-info);
  --tone-fill: color-mix(in oklab, var(--color-info) 14%, transparent);
}

.tone-success {
  --tone: var(--color-success);
  --tone-fill: var(--color-success-muted);
}

.tone-danger {
  --tone: var(--color-error);
  --tone-fill: var(--color-error-muted);
}

.app-icon-button:hover:not(:disabled) {
  background: var(--color-surface-2);
  color: var(--tone);
}

.variant-ghost {
  border-color: var(--color-border-default);
  color: var(--color-fg-secondary);
}

.variant-ghost:not(.tone-neutral) {
  color: var(--tone);
}

.variant-tinted {
  background: var(--tone-fill);
  color: var(--tone);
}

.variant-tinted.tone-neutral {
  color: var(--color-fg-secondary);
}

.variant-tinted:hover:not(:disabled) {
  background: var(--tone-fill);
  border-color: var(--tone);
}

.variant-tinted.tone-neutral:hover:not(:disabled) {
  border-color: var(--color-border-strong);
  color: var(--color-fg-primary);
}

.app-icon-button.is-pressed {
  background: var(--color-amber-muted);
  color: var(--color-amber);
}

.app-icon-button:focus-visible {
  outline: 1px solid var(--color-amber);
  outline-offset: 1px;
}

.app-icon-button:disabled {
  color: var(--color-fg-disabled);
  cursor: default;
}

.variant-tinted:disabled {
  background: var(--color-surface-2);
}

.is-busy {
  cursor: progress;
}

/* Full opacity on purpose, so a running action reads as alive, not as disabled. */
.busy-glyph {
  color: var(--tone);
  animation: spin 0.9s linear infinite;
}

.tone-neutral .busy-glyph {
  color: var(--color-fg-primary);
}

/* Pinned to the top right corner and left unclipped, so a count can sit over the edge. */
.badge {
  position: absolute;
  top: -3px;
  right: -3px;
  pointer-events: none;
}
</style>
