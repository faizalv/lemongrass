<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { ref, useId } from 'vue'
import { AppIconButton, AppInfoPopover } from '@sankasten'
import type { CommitRester } from './resterDraft'
import type { VaultRester } from '../../../../../preload/types'
import ApiSection from './ApiSection.vue'
import WebSection from './WebSection.vue'
import AppSection from './AppSection.vue'

defineProps<{
  rester: VaultRester
  commit: CommitRester
}>()

const infoId = useId()
const anchor = ref<HTMLElement | null>(null)

function show(event: Event): void {
  anchor.value = event.currentTarget as HTMLElement
}

function hide(): void {
  anchor.value = null
}
</script>

<template>
  <div class="inline-section auth-section">
    <div class="users-toolbar auth-section-head">
      <span class="field-label">Authentication</span>
      <AppIconButton
        icon="info"
        size="sm"
        label="About Authentication"
        :aria-describedby="anchor ? infoId : undefined"
        @mouseenter="show"
        @mouseleave="hide"
        @focus="show"
        @blur="hide"
      />
    </div>
    <p class="hint">
      How a model signs in as one of this rester's users. Turn on what the system has.
    </p>
    <AppInfoPopover :id="infoId" :anchor="anchor">
      Each setup tells a channel how to log a user in: the API one for HTTP calls, the web one for
      the app's browser and the Android one for a connected phone. They are shared by every
      environment and every user, so you set each up once. Open a row to edit it. Saving turns it on
      and Turn off removes it.
    </AppInfoPopover>

    <div class="auth-rows">
      <ApiSection :rester="rester" :commit="commit" />
      <WebSection :rester="rester" :commit="commit" />
      <AppSection :rester="rester" :commit="commit" />
    </div>
  </div>
</template>
