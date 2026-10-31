<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { ref } from 'vue'
import AppContextMenu, { type MenuItem } from './AppContextMenu.vue'

// A bare icon that opens a menu of rarer actions, for a surface where those actions must not sit
// as buttons of their own. The icon comes from the caller's slot; the label is its title.
const props = defineProps<{
  items: MenuItem[]
  label: string
  disabled?: boolean
}>()

const emit = defineEmits<{ select: [key: string] }>()

// AppContextMenu's own sizes, to open it above the icon when there is no room below.
const MENU_ROW = 30
const MENU_PAD = 16
const MENU_WIDTH = 200
const GAP = 4

const root = ref<HTMLElement | null>(null)
const menu = ref<{ x: number; y: number } | null>(null)
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
  const height = props.items.length * MENU_ROW + MENU_PAD
  const below = window.innerHeight - rect.bottom - GAP
  const y = below >= height ? rect.bottom + GAP : Math.max(GAP, rect.top - GAP - height)
  // Right-aligned to the icon, which usually sits at the right of a toolbar.
  menu.value = { x: Math.max(GAP, rect.right - MENU_WIDTH), y }
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
    :class="{ open: !!menu }"
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
    <AppContextMenu
      v-if="menu"
      :x="menu.x"
      :y="menu.y"
      :items="items"
      @select="select"
      @close="close"
    />
  </Teleport>
</template>

<style scoped>
.menu-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 3px;
  background: transparent;
  border: none;
  border-radius: var(--radius-sm);
  color: var(--color-fg-muted);
  cursor: pointer;
}

.menu-button:hover:not(:disabled),
.menu-button:focus-visible,
.menu-button.open {
  color: var(--color-fg-accent);
}

.menu-button:focus-visible {
  outline: 1px solid var(--color-amber);
}

.menu-button:disabled {
  color: var(--color-fg-disabled);
  cursor: default;
}
</style>
