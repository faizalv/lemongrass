<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts" generic="T extends string | number">
withDefaults(
  defineProps<{
    options: { value: T; label: string }[]
    modelValue: T
    size?: 'sm' | 'md'
    // Full width of its container with equal segments, for a switch that heads a panel.
    block?: boolean
  }>(),
  { size: 'md', block: false }
)

const emit = defineEmits<{
  'update:modelValue': [value: T]
}>()
</script>

<!-- Picks one of a few modes. Name the group with aria-label or aria-labelledby on the component. -->
<template>
  <div class="segmented" :class="[`size-${size}`, { block }]" role="radiogroup">
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      role="radio"
      :aria-checked="option.value === modelValue"
      @click="emit('update:modelValue', option.value)"
    >
      {{ option.label }}
    </button>
  </div>
</template>

<style scoped>
/* The track is the control height and the 3px inset leaves the pills 6px shorter, so a switch
   lines up with fields and buttons of its size. */
.segmented {
  --h: var(--control-h-md);
  box-sizing: border-box;
  display: inline-flex;
  align-self: flex-start;
  flex-wrap: wrap;
  gap: 2px;
  padding: 3px;
  border-radius: var(--radius-md);
  background: var(--color-surface-3);
}

.segmented.block {
  display: flex;
  align-self: stretch;
  width: 100%;
}

.segmented.block button {
  flex: 1;
}

.segmented.size-sm {
  --h: var(--control-h-sm);
}

.segmented button {
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: calc(var(--h) - 6px);
  padding: 0 var(--space-3);
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-default),
    color var(--duration-fast) var(--ease-default);
}

.segmented button:hover {
  color: var(--color-fg-primary);
}

.segmented button[aria-checked='true'] {
  background: var(--color-surface-1);
  color: var(--color-fg-primary);
  font-weight: var(--weight-semibold);
  box-shadow: var(--shadow-sm);
}

/* Inset, so the ring follows the pill's own edge whatever the track padding and the selected
   pill's shadow. */
.segmented button:focus-visible {
  outline: 1px solid var(--color-amber);
  outline-offset: -1px;
}
</style>
