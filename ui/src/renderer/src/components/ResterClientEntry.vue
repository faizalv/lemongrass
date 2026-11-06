<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppNavItem } from '@sankasten'
import { openResterClient, type ProjectRef } from '../workspace'

const props = defineProps<{
  project: ProjectRef
}>()
</script>

<template>
  <div class="rester-client-panel">
    <AppNavItem size="sm" label="Rester Client" @click="openResterClient(props.project)">
      <template #icon>
        <svg
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M2 4.5h9.5L9.5 2.5" />
          <path d="M12 9.5H2.5l2 2" />
        </svg>
      </template>
    </AppNavItem>
  </div>
</template>

<style scoped>
.rester-client-panel {
  flex-shrink: 0;
  padding: var(--space-2);
  border-top: 1px solid var(--color-border-default);
}
</style>
