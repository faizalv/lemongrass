<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The caveat of a passing connection test, with its fix statement on its own line to copy.
import { onBeforeUnmount, ref } from 'vue'
import { AppBanner, AppIconButton } from '@sankasten'
import type { ConnectionTestNotice } from '../../../../preload/types'

defineProps<{ notice: ConnectionTestNotice }>()

const copied = ref(false)
let copiedTimer: ReturnType<typeof setTimeout> | null = null

async function copyFix(fix: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(fix)
  } catch {
    return
  }
  copied.value = true
  if (copiedTimer) clearTimeout(copiedTimer)
  copiedTimer = setTimeout(() => {
    copied.value = false
  }, 1500)
}

onBeforeUnmount(() => {
  if (copiedTimer) clearTimeout(copiedTimer)
})
</script>

<template>
  <AppBanner class="connection-notice" :title="notice.title">
    <p class="connection-notice-detail">{{ notice.detail }}</p>
    <div v-if="notice.fix" class="connection-notice-fix">
      <code>{{ notice.fix }}</code>
      <AppIconButton
        :icon="copied ? 'check' : 'copy'"
        :label="copied ? 'Copied' : 'Copy the grant'"
        :tone="copied ? 'success' : 'neutral'"
        size="xs"
        @click="copyFix(notice.fix)"
      />
    </div>
  </AppBanner>
</template>
