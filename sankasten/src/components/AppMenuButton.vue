<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { ref } from 'vue'
import type { MenuAnchor } from '../lib/menuPlacement'
import AppContextMenu, { type MenuItem } from './AppContextMenu.vue'

// A bare icon that opens a menu of rarer actions, for a surface where those actions must not sit
// as buttons of their own. The icon comes from the caller's slot; the label is its title.
withDefaults(
  defineProps<{
    items: MenuItem[]
    label: string
    disabled?: boolean
    size?: 'sm' | 'md'
  }>(),
  { disabled: false, size: 'md' }
)

const emit = defineEmits<{ select: [key: string] }>()

const GAP = 4

const root = ref<HTMLElement | null>(null)
const menu = ref<MenuAnchor | null>(null)
// The menu closes on any pointerdown outside it, this icon included, before the icon's click
// arrives; this keeps that click from opening it again.
let closedAt = 0

function toggle(): void {
  if (menu.value) {
    menu.value = null
    return
  }
  if (Date.now() - closedAt < 300) return
  const rect = root.value?.getBoundingClientRect()
  if (!rect) return
  // Right-aligned to the icon, which usually sits at the right of a toolbar, below it or above.
  menu.value = {
    x: { start: rect.left, end: rect.right, prefer: 'backward' },
    y: { start: rect.bottom + GAP, end: rect.top - GAP }
  }
}

function close(): void {
  menu.value = null
  closedAt = Date.now()
}

function select(key: string): void {
  menu.value = null
  emit('select', key)
}
</script>

<template>
  <button
    ref="root"
    type="button"
    class="menu-button"
    :class="[`size-${size}`, { open: !!menu }]"
    :disabled="disabled"
    aria-haspopup="menu"
    :aria-expanded="!!menu"
    :title="label"
    :aria-label="label"
    @click="toggle"
  >
    <slot />
  </button>
  <Teleport to="body">
    <AppContextMenu v-if="menu" :anchor="menu" :items="items" @select="select" @close="close" />
  </Teleport>
</template>

<style scoped>
/* Square at the control height, like AppIconButton, so it sits in a row of icon buttons. */
.menu-button {
  --h: var(--control-h-md);
  --glyph: var(--control-icon-md);
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: var(--h);
  height: var(--h);
  padding: 0;
  background: transparent;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  color: var(--color-fg-muted);
  cursor: pointer;
}

.menu-button.size-sm {
  --h: var(--control-h-sm);
  --glyph: var(--control-icon-sm);
}

.menu-button :deep(svg) {
  height: var(--glyph);
  width: auto;
}

.menu-button:hover:not(:disabled),
.menu-button:focus-visible,
.menu-button.open {
  color: var(--color-fg-accent);
}

.menu-button:hover:not(:disabled),
.menu-button.open {
  background: var(--color-surface-2);
}

.menu-button:focus-visible {
  outline: 1px solid var(--color-amber);
}

.menu-button:disabled {
  color: var(--color-fg-disabled);
  cursor: default;
}
</style>
