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

type Side = 'api' | 'web' | 'app'

// One editor is open at a time, under the cards.
const editing = ref<Side | null>(null)

function toggle(side: Side): void {
  editing.value = editing.value === side ? null : side
}

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
  <section class="auth-section">
    <div class="users-toolbar auth-section-head">
      <span class="field-label">Authentication</span>
      <AppIconButton
        icon="info"
        size="sm"
        label="About Authentication"
        title=""
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
      environment and every user, so you set each up once. Edit opens the form under the cards, so
      the other values stay in view while you type. Saving turns a setup on and Turn off removes it.
    </AppInfoPopover>

    <div class="auth-grid">
      <ApiSection
        :rester="rester"
        :commit="commit"
        :editing="editing === 'api'"
        @toggle="toggle('api')"
        @close="editing = null"
      />
      <WebSection
        :rester="rester"
        :commit="commit"
        :editing="editing === 'web'"
        @toggle="toggle('web')"
        @close="editing = null"
      />
      <AppSection
        :rester="rester"
        :commit="commit"
        :editing="editing === 'app'"
        @toggle="toggle('app')"
        @close="editing = null"
      />
    </div>
  </section>
</template>
