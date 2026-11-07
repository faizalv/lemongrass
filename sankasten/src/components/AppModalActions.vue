<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { inject } from 'vue'
import { modalActionsKey } from './modalActions'

// Inside an AppModal the buttons go to its fixed footer, so they stay reachable however long the
// body is. The footer row exists only after the body mounts, hence the deferred teleport.
const target = inject(modalActionsKey, null)
</script>

<template>
  <Teleport v-if="target" defer :to="target">
    <slot />
  </Teleport>
  <div v-else class="modal-actions-inline">
    <slot />
  </div>
</template>

<style scoped>
.modal-actions-inline {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}
</style>
