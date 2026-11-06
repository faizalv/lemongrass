<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
// A labelled row of live examples inside a component's demo.
defineProps<{ label: string }>()
</script>

<template>
  <div class="demo-row">
    <span class="demo-label">{{ label }}</span>
    <div class="demo-items"><slot /></div>
  </div>
</template>

<style scoped>
.demo-row {
  display: grid;
  grid-template-columns: 120px 1fr;
  align-items: center;
  gap: var(--space-4);
}

.demo-label {
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.demo-items {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
}
</style>
