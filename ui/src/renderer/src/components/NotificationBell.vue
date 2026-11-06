<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppIconButton } from '@sankasten'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import NotificationCenter from './NotificationCenter.vue'
import { askState } from '../asks'
import { approvals } from '../workgroups'
import { center, startNotificationFeed, unreadCount } from '../notifications'

const rootRef = ref<HTMLElement | null>(null)
const count = computed(() => unreadCount())
// A waiting decision is the one thing here that holds an agent up, so it alone colors the badge.
const deciding = computed(() => askState.count.waiting > 0 || approvals.pending.length > 0)
const label = computed(() => {
  if (count.value === 0) return 'Notifications'
  return count.value === 1 ? 'Notifications, 1 new' : `Notifications, ${count.value} new`
})

function toggle(): void {
  center.open = !center.open
}

function onDocumentClick(event: MouseEvent): void {
  if (center.open && rootRef.value && !rootRef.value.contains(event.target as Node)) {
    center.open = false
  }
}

function onKey(event: KeyboardEvent): void {
  if (center.open && event.key === 'Escape') center.open = false
}

onMounted(() => {
  startNotificationFeed()
  document.addEventListener('click', onDocumentClick, true)
  document.addEventListener('keydown', onKey)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocumentClick, true)
  document.removeEventListener('keydown', onKey)
})
</script>

<template>
  <div ref="rootRef" class="bell-zone">
    <AppIconButton
      :label="label"
      size="sm"
      :pressed="center.open"
      :aria-expanded="center.open"
      @click="toggle"
    >
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.6"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9" />
        <path d="M10.3 21a1.94 1.94 0 0 0 3.4 0" />
      </svg>
      <template v-if="count > 0" #badge>
        <span class="count" :class="{ deciding }">
          {{ count > 99 ? '99+' : count }}
        </span>
      </template>
    </AppIconButton>
    <NotificationCenter v-if="center.open" />
  </div>
</template>

<style scoped>
.bell-zone {
  position: relative;
  display: flex;
  align-items: center;
}

.count {
  display: block;
  min-width: 15px;
  padding: 0 4px;
  border-radius: var(--radius-pill);
  background: var(--color-gray-500);
  color: var(--color-white);
  font-size: 10px;
  font-weight: var(--weight-semibold);
  line-height: 15px;
  text-align: center;
}

.count.deciding {
  background: var(--color-amber);
  color: var(--color-black);
}
</style>
