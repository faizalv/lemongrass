<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { provide } from 'vue'
import { MOSAIC_KEY } from './mosaic'

// Lays AppCapsules out as a mosaic: they wrap, and each grows to fill its row, so every row is
// justified while widths follow the content. One capsule is expanded at a time; it takes a row of
// its own and pushes the rest down.
const expanded = defineModel<string | null>('expanded', { default: null })

function toggle(value: string): void {
  expanded.value = expanded.value === value ? null : value
}

provide(MOSAIC_KEY, { expanded, toggle })
</script>

<template>
  <div class="app-mosaic">
    <slot />
  </div>
</template>

<style scoped>
.app-mosaic {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1);
}
</style>
