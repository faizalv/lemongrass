<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppNavItem } from '@sankasten'
import { openDevice, type ProjectRef } from '../workspace'

const props = defineProps<{
  project: ProjectRef
}>()
</script>

<template>
  <div class="device-panel">
    <AppNavItem size="sm" label="Device" @click="openDevice(props.project)">
      <template #icon>
        <svg
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <rect x="3.5" y="1.5" width="7" height="11" rx="1.4" />
          <path d="M6 10.5h2" />
        </svg>
      </template>
    </AppNavItem>
  </div>
</template>

<style scoped>
.device-panel {
  flex-shrink: 0;
  padding: var(--space-2);
  border-top: 1px solid var(--color-border-default);
}
</style>
