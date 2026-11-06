<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import AppBadge from './AppBadge.vue'
import AppIcon from './AppIcon.vue'

// The header of a foldable group: a chevron that turns while open, a label and an optional count.
// It is a row heading, so its height comes from its text, not the control scale. Anything else in
// the same row, such as an action button or a note, stays outside it.
withDefaults(
  defineProps<{
    label?: string
    count?: number
    // text is a muted number after the label; pill is an AppBadge in countTone.
    countLook?: 'text' | 'pill'
    countTone?: 'accent' | 'info' | 'success' | 'danger' | 'neutral'
    // Full width with a little padding, for a group head in a list; otherwise sized to content.
    block?: boolean
  }>(),
  {
    label: undefined,
    count: undefined,
    countLook: 'text',
    countTone: 'info',
    block: false
  }
)

const open = defineModel<boolean>('open', { default: false })
</script>

<template>
  <button
    type="button"
    class="app-disclosure"
    :class="{ block }"
    :aria-expanded="open"
    @click="open = !open"
  >
    <AppIcon class="chevron" :class="{ open }" name="chevronRight" :size="9" />
    <span class="label"
      ><slot>{{ label }}</slot></span
    >
    <template v-if="count !== undefined">
      <AppBadge v-if="countLook === 'pill'" :tone="countTone" size="sm">{{ count }}</AppBadge>
      <span v-else class="count">{{ count }}</span>
    </template>
  </button>
</template>

<style scoped>
.app-disclosure {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  padding: 0;
  background: none;
  border: none;
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
  text-align: left;
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-out);
}

.app-disclosure.block {
  display: flex;
  width: 100%;
  padding: 3px var(--space-2);
}

.app-disclosure:hover {
  color: var(--color-fg-primary);
}

.app-disclosure:focus-visible {
  outline: 1px solid var(--color-amber);
  outline-offset: 1px;
  border-radius: var(--radius-sm);
}

.chevron {
  flex-shrink: 0;
  transition: transform var(--duration-fast) var(--ease-out);
}

.chevron.open {
  transform: rotate(90deg);
}

.label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.count {
  flex-shrink: 0;
  color: var(--color-fg-muted);
  font-weight: var(--weight-regular);
}
</style>
