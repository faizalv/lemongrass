<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppNavItem } from '@sankasten'
import { askState } from '../asks'
import { vaultSession } from '../vaultSession'

const emit = defineEmits<{
  open: []
}>()
</script>

<template>
  <div class="vault-panel">
    <AppNavItem
      size="sm"
      label="Vault"
      :state="vaultSession.unlocked ? 'Unlocked' : 'Locked'"
      :active="vaultSession.unlocked"
      :count="askState.count.waiting"
      :count-title="
        askState.count.waiting === 1
          ? 'A decision is waiting for you'
          : `${askState.count.waiting} decisions are waiting for you`
      "
      @click="emit('open')"
    >
      <template #icon>
        <svg
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <rect x="2.5" y="6.5" width="9" height="6" rx="1.2" />
          <path d="M4.5 6.5V4.5a2.5 2.5 0 0 1 5 0v2" />
          <circle cx="7" cy="9.5" r="0.8" />
        </svg>
      </template>
    </AppNavItem>
  </div>
</template>

<style scoped>
.vault-panel {
  flex-shrink: 0;
  padding: var(--space-2);
  border-top: 1px solid var(--color-border-default);
}
</style>
