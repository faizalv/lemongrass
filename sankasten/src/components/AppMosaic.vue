<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { provide } from 'vue'
import { MOSAIC_KEY } from './mosaic'

// Lays AppCapsules out as a mosaic: they wrap, and each grows to fill its row, so every row is
// justified while widths follow the content. One capsule is expanded at a time; it takes a row of
// its own and pushes the rest down.
const expanded = defineModel<string | null>('expanded', { default: null })

// fill is the default: every capsule grows to fill its row. Without it a capsule keeps the width
// of its content, for a short row where a lone capsule stretched across the page reads as a bar;
// an expanded capsule still takes a row of its own.
withDefaults(defineProps<{ fill?: boolean }>(), { fill: true })

function toggle(value: string): void {
  expanded.value = expanded.value === value ? null : value
}

provide(MOSAIC_KEY, { expanded, toggle })
</script>

<template>
  <div class="app-mosaic" :class="{ natural: !fill }">
    <slot />
  </div>
</template>

<style scoped>
.app-mosaic {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1);
}
.app-mosaic.natural :deep(.app-capsule:not(.expanded)) {
  flex: 0 1 auto;
}
</style>
