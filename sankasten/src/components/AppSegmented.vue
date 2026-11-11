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
/* The track is the control height and the segments fill it edge to edge, so a switch lines up
   with fields and buttons of its size and the selected fill covers exactly its share. An inset
   pill looked smaller than the unselected segments, whose share the eye reads out to the track's
   edge. */
.segmented {
  --h: var(--control-h-md);
  box-sizing: border-box;
  display: inline-flex;
  align-self: flex-start;
  flex-wrap: wrap;
  border-radius: var(--radius-md);
  background: var(--color-surface-3);
}

/* No width: 100%, which would add to a caller's margin and overflow; a flex child stretches and a
   block-level flex box fills its container either way. */
.segmented.block {
  display: flex;
  align-self: stretch;
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
  height: var(--h);
  padding: 0 var(--space-3);
  border: none;
  border-radius: var(--radius-md);
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

/* The focus color marks the selected segment as a solid fill, like a primary button: a muted fill
   washes out on the track's raised surface. After :hover, so hovering keeps the selected look. */
.segmented button[aria-checked='true'] {
  background: var(--color-amber);
  color: var(--color-on-fill);
  font-weight: var(--weight-semibold);
}

/* Inset, so the ring follows the segment's own edge inside the track. */
.segmented button:focus-visible {
  outline: 1px solid var(--color-amber);
  outline-offset: -1px;
}

/* An amber ring would vanish into the selected segment's amber fill. */
.segmented button[aria-checked='true']:focus-visible {
  outline-color: var(--color-on-fill);
}
</style>
