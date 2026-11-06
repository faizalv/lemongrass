<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppNavItem } from '@sankasten'
import { sshIcon } from '../icons'
import { openSSH, type ProjectRef } from '../workspace'

const props = defineProps<{
  project: ProjectRef
}>()
</script>

<template>
  <div class="ssh-entry">
    <AppNavItem size="sm" label="SSH" @click="openSSH(props.project)">
      <template #icon>
        <svg
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <component :is="shape.tag" v-for="(shape, i) in sshIcon" :key="i" v-bind="shape.attrs" />
        </svg>
      </template>
    </AppNavItem>
  </div>
</template>

<style scoped>
.ssh-entry {
  flex-shrink: 0;
  padding: var(--space-2);
  border-top: 1px solid var(--color-border-default);
}
</style>
