<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The resolver's icons in one map, as GitIcon holds the git panel's, so a toolbar control is a
// name here and never its own inline SVG.
const SHAPES = {
  prev: ['M3.5 8.5L7 5l3.5 3.5'],
  next: ['M3.5 5.5L7 9l3.5-3.5'],
  focus: ['M2 5V2h3M9 2h3v3M12 9v3H9M5 12H2V9', 'M7 5.6a1.4 1.4 0 1 1 0 2.8a1.4 1.4 0 1 1 0-2.8'],
  more: [
    'M3 6.3a.7.7 0 1 1 0 1.4a.7.7 0 1 1 0-1.4',
    'M7 6.3a.7.7 0 1 1 0 1.4a.7.7 0 1 1 0-1.4',
    'M11 6.3a.7.7 0 1 1 0 1.4a.7.7 0 1 1 0-1.4'
  ],
  accept: ['M3 7.5l2.5 2.5L11 4.5'],
  acceptAll: ['M1.5 7.5L4 10l5-5.5', 'M7 9.5l.5.5 5-5.5']
} as const

export type ConflictIconName = keyof typeof SHAPES

defineProps<{
  name: ConflictIconName
}>()
</script>

<template>
  <svg
    class="conflict-icon"
    width="13"
    height="13"
    viewBox="0 0 14 14"
    fill="none"
    stroke="currentColor"
    stroke-width="1.3"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path v-for="d in SHAPES[name]" :key="d" :d="d" />
  </svg>
</template>

<style scoped>
.conflict-icon {
  flex-shrink: 0;
}
</style>
