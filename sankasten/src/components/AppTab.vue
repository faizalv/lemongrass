<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import AppTabClose from './AppTabClose.vue'

// A small tab in a strip of open items, such as files in an editor: an optional icon, the label
// ellipsized, and a close mark that confirms while the item is dirty. The caller lays tabs out in
// a row with role="tablist"; the active one is underlined in amber.
withDefaults(
  defineProps<{
    label: string
    // The tooltip, such as a full path; the label when absent.
    title?: string
    active?: boolean
    dirty?: boolean
    // The close mark's name, such as "Close notes.txt".
    closeLabel: string
    confirmLabel?: string
  }>(),
  { title: undefined, active: false, dirty: false, confirmLabel: 'Close without saving' }
)

const emit = defineEmits<{ select: []; close: [] }>()

function onKey(event: KeyboardEvent): void {
  if (event.target !== event.currentTarget) return
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    emit('select')
  }
}
</script>

<template>
  <div
    class="app-tab"
    :class="{ active }"
    role="tab"
    tabindex="0"
    :aria-selected="active"
    :title="title ?? label"
    @click="emit('select')"
    @keydown="onKey"
  >
    <span v-if="$slots.icon" class="icon"><slot name="icon" /></span>
    <span class="label">{{ label }}</span>
    <AppTabClose
      size="sm"
      :label="closeLabel"
      :dirty="dirty"
      :confirm-label="confirmLabel"
      @close="emit('close')"
    />
  </div>
</template>

<style scoped>
.app-tab {
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  max-width: 200px;
  height: 100%;
  min-height: var(--control-h-sm);
  padding: 0 var(--space-1) 0 var(--space-2);
  border-bottom: 2px solid transparent;
  color: var(--color-fg-muted);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  cursor: pointer;
  user-select: none;
  transition: color var(--duration-fast) var(--ease-out);
}

.app-tab:hover {
  color: var(--color-fg-primary);
}

.app-tab.active {
  border-bottom-color: var(--color-amber);
  color: var(--color-fg-primary);
}

.app-tab:focus-visible {
  outline: 1px solid var(--color-amber);
  outline-offset: -1px;
}

.icon {
  display: inline-flex;
  flex-shrink: 0;
}

.label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
