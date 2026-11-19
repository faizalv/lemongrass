<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import AppIcon from './AppIcon.vue'

withDefaults(defineProps<{ size?: number }>(), { size: 13 })
</script>

<!-- A turning spinner glyph in the holder's text color, for a short wait inside a control or a line. -->
<template>
  <span class="app-spinner" aria-hidden="true">
    <AppIcon name="spinner" :size="size" />
  </span>
</template>

<style scoped>
/* The turn sits on this HTML box, not on the svg: Chromium runs an SVG element's animation on the
   main thread every frame, while an HTML box's transform animation runs on the compositor. */
.app-spinner {
  display: inline-flex;
  flex: none;
  animation: spin 0.9s linear infinite;
}
</style>
