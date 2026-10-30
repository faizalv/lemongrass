<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import type { ConflictTake } from '../../conflictModel'
import AppButton from '../AppButton.vue'

// Mounted by the middle pane in a block widget above each conflict, so its height must stay the
// one the side panes' bars use, or the three panes drift apart.
defineProps<{
  label: string
  resolved: boolean
}>()

const emit = defineEmits<{ take: [side: ConflictTake] }>()
</script>

<template>
  <div class="chunk-bar">
    <span class="label">{{ label }}</span>
    <span v-if="resolved" class="state">Resolved</span>
    <AppButton class="take" @click="emit('take', 'left')">Take left</AppButton>
    <AppButton class="take" @click="emit('take', 'right')">Take right</AppButton>
    <AppButton class="take" title="Left, then right" @click="emit('take', 'both')">
      Take both
    </AppButton>
  </div>
</template>

<style scoped>
.chunk-bar {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  height: 100%;
}

.label {
  color: var(--color-fg-secondary);
}

.state {
  color: var(--color-success);
}

.take {
  padding: 0 var(--space-3);
  line-height: 20px;
}

.take:first-of-type {
  margin-left: auto;
}
</style>
