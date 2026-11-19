<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// Sized in em so a mark set inside a line of text scales with that text.
defineProps<{ d: string; box: number; label: string }>()
</script>

<template>
  <svg class="mark" :viewBox="`0 0 ${box} ${box}`" role="img" :aria-label="label" fill="currentColor">
    <title>{{ label }}</title>
    <path :d="d" />
  </svg>
</template>

<style scoped>
.mark {
  display: inline-block;
  width: 1em;
  height: 1em;
  flex: none;
}
</style>
